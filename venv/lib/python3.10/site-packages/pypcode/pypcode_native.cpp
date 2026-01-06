#include <cstdio>
#include <optional>
#include <string>
#include <unordered_set>

#include <nanobind/nanobind.h>
#include <nanobind/stl/map.h>
#include <nanobind/stl/optional.h>
#include <nanobind/stl/string.h>
#include <nanobind/stl/unique_ptr.h>
#include <nanobind/stl/vector.h>

#include "sleigh/error.hh"
#include "sleigh/loadimage.hh"
#include "sleigh/opcodes.hh"
#include "sleigh/sleigh.hh"
#include "sleigh/space.hh"
#include "sleigh/translate.hh"
#include "sleigh/xml.hh"

namespace nb = nanobind;
using namespace nb::literals;
using namespace ghidra;

const char *
#include "__version__.py"
    ;

// #define DEBUG 1

#ifndef DEBUG
#define DEBUG 0
#endif

#if DEBUG
#define LOG(fmt, ...) fprintf(stderr, "pypcode_native: " fmt "\n", ##__VA_ARGS__);
#else
#define LOG(fmt, ...) \
    do {              \
    } while (0)
#endif

#define MIN(x, y) ((x) < (y) ? (x) : (y))

struct PcodeOp {
    OpCode m_opcode;
    std::optional<VarnodeData> m_output;
    std::vector<VarnodeData> m_inputs;
};

class ContextPypcode : public ContextInternal {
    bool m_finalized;
    std::unordered_set<string> m_variables;

public:
    ContextPypcode() : ContextInternal()
    {
        m_finalized = false;
    }

    void finalize()
    {
        m_finalized = true;
    }

    virtual void registerVariable(const string &nm, int4 sbit, int4 ebit)
    {
        if (!m_finalized) {
            ContextInternal::registerVariable(nm, sbit, ebit);
            m_variables.insert(nm);
        }
    }

    void resetAllVariables()
    {
        for (const string &nm : m_variables) {
            auto val = ContextDatabase::getDefaultValue(nm);
            setVariableRegion(nm, Address(Address::m_minimal), Address(), val);
        }
    }
};

class SimpleLoadImage : public LoadImage {
    uintb m_baseaddr;
    int4 m_length;
    const unsigned char *m_data;

public:
    SimpleLoadImage() : LoadImage("nofile")
    {
        m_baseaddr = 0;
        m_data = NULL;
        m_length = 0;
    }

    void setData(uintb ad, const unsigned char *ptr, int4 sz)
    {
        m_baseaddr = ad;
        m_data = ptr;
        m_length = sz;
    }

    void loadFill(uint1 *ptr, int4 size, const Address &addr)
    {
        LOG("Filling %d bytes at %lx", size, addr.getOffset());
        uintb start = addr.getOffset();
        uintb max = m_baseaddr + m_length - 1;

        //
        // When decoding an instruction, SLEIGH will attempt to pull in several
        // bytes at a time, starting at each instruction boundary.
        //
        // If the start address is outside of the defined range, bail out.
        // Otherwise, if we have some data to provide but cannot satisfy the
        // entire request, fill the remainder of the buffer with zero.
        //
        if (start > max || start < m_baseaddr) {
            throw std::out_of_range("Attempting to lift outside buffer range");
        }

        for (int4 i = 0; i < size; i++) {
            uintb curoff = start + i;
            if ((curoff < m_baseaddr) || (curoff > max)) {
                ptr[i] = 0;
                continue;
            }
            uintb diff = curoff - m_baseaddr;
            ptr[i] = m_data[(int4)diff];
        }
    }

    virtual string getArchType(void) const
    {
        return "myload";
    }
    virtual void adjustVma(long adjust)
    {
    }
};

class PcodeEmitCacher : public PcodeEmit {
public:
    std::vector<PcodeOp> m_ops;
    bool m_bb_terminating_op_emitted;

    PcodeEmitCacher() : m_bb_terminating_op_emitted(false)
    {
        m_ops.reserve(512);
    }

    // Encode P-code ops into csleigh structures and append them to the translation buffer
    void dump(const Address &addr, OpCode opc, VarnodeData *outvar, VarnodeData *invars, int4 num_invars)
    {
        LOG("Emitting pcode op %d with %d-in,%d-out varnodes from %lx",
            opc,
            num_invars,
            outvar ? 1 : 0,
            addr.getOffset());
        m_bb_terminating_op_emitted |= opc == CPUI_BRANCH || opc == CPUI_CBRANCH || opc == CPUI_BRANCHIND ||
                                       opc == CPUI_RETURN || opc == CPUI_CALL || opc == CPUI_CALLIND;

        m_ops.emplace_back();
        PcodeOp &op = m_ops.back();

        op.m_opcode = opc;
        if (outvar) {
            op.m_output.emplace(*outvar);
        }
        op.m_inputs.reserve(num_invars);
        for (int i = 0; i < num_invars; i++) {
            op.m_inputs.emplace_back(invars[i]);
        }
    }
};

struct DisassemblyInstruction {
    Address m_addr;
    uint64_t m_length;
    std::string m_mnem;
    std::string m_body;
};

class AssemblyEmitCacher : public AssemblyEmit {
public:
    DisassemblyInstruction &m_disas;

    AssemblyEmitCacher(DisassemblyInstruction &disas) : m_disas(disas)
    {
    }

    void dump(const Address &addr, const std::string &mnem, const std::string &body)
    {
        m_disas.m_addr = addr;
        m_disas.m_mnem = mnem;
        m_disas.m_body = body;
    };
};

class Disassembly {
public:
    std::vector<DisassemblyInstruction> m_instructions;

    Disassembly()
    {
        LOG("Disassembly %p created", this);
    }

    Disassembly(Disassembly &&o) noexcept : m_instructions(std::move(o.m_instructions))
    {
        LOG("Disassembly moved from %p to %p", &o, this);
    }

    ~Disassembly()
    {
        LOG("Disassembly %p released", this);
    }
};

class Translation {
public:
    std::vector<PcodeOp> m_ops;

    Translation()
    {
        LOG("Translation %p created", this);
    }

    Translation(Translation &&o) noexcept : m_ops(std::move(o.m_ops))
    {
        LOG("Translation moved from %p to %p", &o, this);
    }

    ~Translation()
    {
        LOG("Translation %p released", this);
    }
};

enum TranslateFlags {
    BB_TERMINATING = 1,
};

class Context {
public:
    SimpleLoadImage m_loader;
    ContextPypcode m_context_db;
    DocumentStorage m_document_storage;
    Document *m_document;
    Element *m_tags;
    std::unique_ptr<Sleigh> m_sleigh;

    Context(const std::string &path)
    {
        LOG("Context %p created", this);

        // FIXME: Globals...
        AttributeId::initialize();
        ElementId::initialize();

        LOG("%p Loading slafile...", this);
        istringstream sleighfilename(path);
        m_document = m_document_storage.parseDocument(sleighfilename);
        m_tags = m_document->getRoot();
        m_document_storage.registerTag(m_tags);

        LOG("Setting up translator");
        m_sleigh.reset(new Sleigh(&m_loader, &m_context_db));
        m_sleigh->initialize(m_document_storage);
        m_context_db.finalize();
    }

    ~Context()
    {
        LOG("Context %p released", this);
    }

    void reset(void)
    {
        m_sleigh.reset(new Sleigh(&m_loader, &m_context_db));
        m_sleigh->initialize(m_document_storage);
        m_context_db.finalize();
    }

    std::unique_ptr<Disassembly>
    disassemble(const char *bytes, unsigned int num_bytes, uint64_t address, unsigned int max_instructions)
    {
        LOG("%p Disassembling bytes=%p, num_bytes=%d, address=%lx", this, bytes, num_bytes, address);
        std::unique_ptr<Disassembly> disassembly(new Disassembly());
        int num_instructions = 0;
        uint32_t offset = 0;

        m_sleigh->fastReset();
        m_loader.setData(address, (const unsigned char *)bytes, num_bytes);
        disassembly->m_instructions.reserve(10);

        while ((offset < num_bytes) && (!max_instructions || (num_instructions < max_instructions))) {
            Address addr(m_sleigh->getDefaultCodeSpace(), address + offset);

            disassembly->m_instructions.emplace_back();
            DisassemblyInstruction &ins = disassembly->m_instructions.back();

            AssemblyEmitCacher asm_cache(ins);

            // Disassemble the next instruction. If an error occurs after successful disassembly of at least one
            // instruction, suppress the error and return the successful disassembly. If the caller attempts
            // disassembly again at the position where the error occurred, then propagate the error.
            try {
                ins.m_length = m_sleigh->printAssembly(asm_cache, addr);
            } catch (BadDataError &err) {
                if (offset) {
                    disassembly->m_instructions.resize(num_instructions);
                    break;
                }
                throw err;
            }

            num_instructions += 1;
            offset += ins.m_length;
        }

        return disassembly;
    }

    std::unique_ptr<Translation> translate(const char *bytes,
                                           unsigned int num_bytes,
                                           uint64_t base_address,
                                           unsigned int max_instructions,
                                           uint32_t flags)
    {
        LOG("%p Translating bytes=%p, num_bytes=%d, base_address=0x%lx, max_instructions=%d flags=0x%x",
            this,
            bytes,
            num_bytes,
            base_address,
            max_instructions,
            flags);
        std::unique_ptr<Translation> translation(new Translation);
        PcodeEmitCacher pcode_cache;
        uint32_t offset = 0;

        m_sleigh->fastReset();
        m_loader.setData(base_address, (const unsigned char *)bytes, num_bytes);

        int num_instructions = 0;
        while ((offset < num_bytes) && (!max_instructions || (num_instructions < max_instructions))) {
            Address addr(m_sleigh->getDefaultCodeSpace(), base_address + offset);
            LOG("Lifting at 0x%lx+0x%x=0x%lx", base_address, offset, base_address + offset);

            int imark_idx = pcode_cache.m_ops.size();
            pcode_cache.m_ops.emplace_back();

            // Translate the next instruction. If an error occurs after successful translation of at least one
            // instruction, suppress the error and return the successful translation. If the caller attempts
            // translation again at the position where the error occurred, then propagate the error.
            uint32_t num_bytes_decoded = 0;
            try {
                num_bytes_decoded = m_sleigh->oneInstruction(pcode_cache, addr);
            } catch (BadDataError &err) {
                if (offset) {
                    pcode_cache.m_ops.resize(imark_idx);
                    break;
                }
                throw err;
            } catch (UnimplError &err) {
                if (offset) {
                    pcode_cache.m_ops.resize(imark_idx);
                    break;
                }
                throw err;
            }

            PcodeOp &imark_op = pcode_cache.m_ops[imark_idx];
            imark_op.m_opcode = OpCode::CPUI_IMARK;

            // Add varnode to imark op for every decoded instruction in this translation
            for (int sum = 0; sum < num_bytes_decoded;) {
                imark_op.m_inputs.emplace_back();
                VarnodeData &imark_vn = imark_op.m_inputs.back();
                imark_vn.space = addr.getSpace();
                imark_vn.offset = addr.getOffset() + sum;
                imark_vn.size = m_sleigh->instructionLength(addr);

                sum += imark_vn.size;
                num_instructions++;
            }

            if ((flags & TranslateFlags::BB_TERMINATING) && pcode_cache.m_bb_terminating_op_emitted) {
                LOG("Reached end of block");
                break;
            }

            offset += num_bytes_decoded;
        }

        translation->m_ops = std::move(pcode_cache.m_ops);
        return translation;
    }
};

NB_MODULE(pypcode_native, m)
{
    m.attr("__version__") = __version__;

    m.doc() = "pypcode native extension providing machine code disassembly and translation to P-Code.";

    nb::exception<LowlevelError>(m, "LowlevelError"); // "The lowest level error"
    nb::exception<BadDataError>(m, "BadDataError"); // "Exception for bad instruction data"
    nb::exception<UnimplError>(m, "UnimplError"); // "Exception for encountering unimplemented P-Code"
    nb::exception<DecoderError>(m, "DecoderError"); // "An exception thrown by the XML parser"

    nb::class_<AddrSpace>(m, "AddrSpace", "A region where processor data is stored.")
        .def_prop_ro(
            "name", [](AddrSpace &as) { return as.getName(); }, "name(self) -> str\nThe name of this address space.");

    nb::class_<Address>(m, "Address", "Low level machine byte address.")
        .def_prop_ro(
            "space",
            [](Address &a) { return a.getSpace(); },
            nb::rv_policy::reference_internal,
            "space(self) -> AddrSpace\nThe address space.")
        .def_prop_ro(
            "offset", [](Address &a) { return a.getOffset(); }, "offset(self) -> int\nThe offset within the space.");

    nb::class_<VarnodeData>(m, "Varnode", "Data defining a specific memory location.")
        .def(nb::init<>())
        .def_ro("space",
                &VarnodeData::space,
                nb::rv_policy::reference_internal,
                "space(self) -> AddrSpace\nThe address space.")
        .def_ro("offset", &VarnodeData::offset, "offset(self) -> int\nThe offset within the space.")
        .def_ro("size", &VarnodeData::size, "size(self) -> int\nThe number of bytes in the location.")
        .def(
            "getRegisterName",
            [](VarnodeData &a) { return a.space->getTrans()->getRegisterName(a.space, a.offset, a.size); },
            "Return the register name if this Varnode references a register, otherwise return the empty string.")
        .def(
            "getUserDefinedOpName",
            [](VarnodeData &a) {
                vector<string> userops;
                a.space->getTrans()->getUserOpNames(userops);
                if (a.offset >= userops.size()) {
                    throw std::out_of_range("index out of range");
                }
                return userops[a.offset];
            },
            "Get the name of a user defined operation.")
        .def(
            "getSpaceFromConst",
            [](VarnodeData &a) { return a.getSpaceFromConst(); },
            nb::rv_policy::reference_internal,
            "Recover encoded address space from constant value.");

    nb::enum_<OpCode>(m, "OpCode", "OpCode defining a specific p-code operation.")
        .value("IMARK", OpCode::CPUI_IMARK, "Instruction marker")
        .value("COPY", OpCode::CPUI_COPY, "Copy one operand to another")
        .value("LOAD", OpCode::CPUI_LOAD, "Load from a pointer into a specified address space")
        .value("STORE", OpCode::CPUI_STORE, "Store at a pointer into a specified address space")
        .value("BRANCH", OpCode::CPUI_BRANCH, "Always branch")
        .value("CBRANCH", OpCode::CPUI_CBRANCH, "Conditional branch")
        .value("BRANCHIND", OpCode::CPUI_BRANCHIND, "Indirect branch (jumptable)")
        .value("CALL", OpCode::CPUI_CALL, "Call to an absolute address")
        .value("CALLIND", OpCode::CPUI_CALLIND, "Call through an indirect address")
        .value("CALLOTHER", OpCode::CPUI_CALLOTHER, "User-defined operation")
        .value("RETURN", OpCode::CPUI_RETURN, "Return from subroutine")
        .value("INT_EQUAL", OpCode::CPUI_INT_EQUAL, "Integer comparison, equality (==)")
        .value("INT_NOTEQUAL", OpCode::CPUI_INT_NOTEQUAL, "Integer comparison, in-equality (!=)")
        .value("INT_SLESS", OpCode::CPUI_INT_SLESS, "Integer comparison, signed less-than (<)")
        .value("INT_SLESSEQUAL", OpCode::CPUI_INT_SLESSEQUAL, "Integer comparison, signed less-than-or-equal (<=)")
        .value("INT_LESS", OpCode::CPUI_INT_LESS, "Integer comparison, unsigned less-than (<)")
        .value("INT_LESSEQUAL", OpCode::CPUI_INT_LESSEQUAL, "Integer comparison, unsigned less-than-or-equal (<=)")
        .value("INT_ZEXT", OpCode::CPUI_INT_ZEXT, "Zero extension")
        .value("INT_SEXT", OpCode::CPUI_INT_SEXT, "Sign extension")
        .value("INT_ADD", OpCode::CPUI_INT_ADD, "Addition, signed or unsigned (+)")
        .value("INT_SUB", OpCode::CPUI_INT_SUB, "Subtraction, signed or unsigned (-)")
        .value("INT_CARRY", OpCode::CPUI_INT_CARRY, "Test for unsigned carry")
        .value("INT_SCARRY", OpCode::CPUI_INT_SCARRY, "Test for signed carry")
        .value("INT_SBORROW", OpCode::CPUI_INT_SBORROW, "Test for signed borrow")
        .value("INT_2COMP", OpCode::CPUI_INT_2COMP, "Twos complement")
        .value("INT_NEGATE", OpCode::CPUI_INT_NEGATE, "Logical/bitwise negation (~)")
        .value("INT_XOR", OpCode::CPUI_INT_XOR, "Logical/bitwise exclusive-or (^)")
        .value("INT_AND", OpCode::CPUI_INT_AND, "Logical/bitwise and (&)")
        .value("INT_OR", OpCode::CPUI_INT_OR, "Logical/bitwise or (|)")
        .value("INT_LEFT", OpCode::CPUI_INT_LEFT, "Left shift (<<)")
        .value("INT_RIGHT", OpCode::CPUI_INT_RIGHT, "Right shift, logical (>>)")
        .value("INT_SRIGHT", OpCode::CPUI_INT_SRIGHT, "Right shift, arithmetic (>>)")
        .value("INT_MULT", OpCode::CPUI_INT_MULT, "Integer multiplication, signed and unsigned (*)")
        .value("INT_DIV", OpCode::CPUI_INT_DIV, "Integer division, unsigned (/)")
        .value("INT_SDIV", OpCode::CPUI_INT_SDIV, "Integer division, signed (/)")
        .value("INT_REM", OpCode::CPUI_INT_REM, "Remainder/modulo, unsigned (%)")
        .value("INT_SREM", OpCode::CPUI_INT_SREM, "Remainder/modulo, signed (%)")
        .value("BOOL_NEGATE", OpCode::CPUI_BOOL_NEGATE, "Boolean negate (!)")
        .value("BOOL_XOR", OpCode::CPUI_BOOL_XOR, "Boolean exclusive-or (^^)")
        .value("BOOL_AND", OpCode::CPUI_BOOL_AND, "Boolean and (&&)")
        .value("BOOL_OR", OpCode::CPUI_BOOL_OR, "Boolean or (||)")
        .value("FLOAT_EQUAL", OpCode::CPUI_FLOAT_EQUAL, "Floating-point comparison, equality (==)")
        .value("FLOAT_NOTEQUAL", OpCode::CPUI_FLOAT_NOTEQUAL, "Floating-point comparison, in-equality (!=)")
        .value("FLOAT_LESS", OpCode::CPUI_FLOAT_LESS, "Floating-point comparison, less-than (<)")
        .value("FLOAT_LESSEQUAL", OpCode::CPUI_FLOAT_LESSEQUAL, "Floating-point comparison, less-than-or-equal (<=)")
        .value("FLOAT_NAN", OpCode::CPUI_FLOAT_NAN, "Not-a-number test (NaN)")
        .value("FLOAT_ADD", OpCode::CPUI_FLOAT_ADD, "Floating-point addition (+)")
        .value("FLOAT_DIV", OpCode::CPUI_FLOAT_DIV, "Floating-point division (/)")
        .value("FLOAT_MULT", OpCode::CPUI_FLOAT_MULT, "Floating-point multiplication (*)")
        .value("FLOAT_SUB", OpCode::CPUI_FLOAT_SUB, "Floating-point subtraction (-)")
        .value("FLOAT_NEG", OpCode::CPUI_FLOAT_NEG, "Floating-point negation (-)")
        .value("FLOAT_ABS", OpCode::CPUI_FLOAT_ABS, "Floating-point absolute value (abs)")
        .value("FLOAT_SQRT", OpCode::CPUI_FLOAT_SQRT, "Floating-point square root (sqrt)")
        .value("FLOAT_INT2FLOAT", OpCode::CPUI_FLOAT_INT2FLOAT, "Convert an integer to a floating-point")
        .value("FLOAT_FLOAT2FLOAT", OpCode::CPUI_FLOAT_FLOAT2FLOAT, "Convert between different floating-point sizes")
        .value("FLOAT_TRUNC", OpCode::CPUI_FLOAT_TRUNC, "Round towards zero")
        .value("FLOAT_CEIL", OpCode::CPUI_FLOAT_CEIL, "Round towards +infinity")
        .value("FLOAT_FLOOR", OpCode::CPUI_FLOAT_FLOOR, "Round towards -infinity")
        .value("FLOAT_ROUND", OpCode::CPUI_FLOAT_ROUND, "Round towards nearest")
        .value("MULTIEQUAL", OpCode::CPUI_MULTIEQUAL, "Phi-node operator")
        .value("INDIRECT", OpCode::CPUI_INDIRECT, "Copy with an indirect effect")
        .value("PIECE", OpCode::CPUI_PIECE, "Concatenate")
        .value("SUBPIECE", OpCode::CPUI_SUBPIECE, "Truncate")
        .value("CAST", OpCode::CPUI_CAST, "Cast from one data-type to another")
        .value("PTRADD", OpCode::CPUI_PTRADD, "Index into an array ([])")
        .value("PTRSUB", OpCode::CPUI_PTRSUB, "Drill down to a sub-field  (->)")
        .value("SEGMENTOP", OpCode::CPUI_SEGMENTOP, "Look-up a segmented address")
        .value("CPOOLREF", OpCode::CPUI_CPOOLREF, "Recover a value from the constant pool")
        .value("NEW", OpCode::CPUI_NEW, "Allocate a new object (new)")
        .value("INSERT", OpCode::CPUI_INSERT, "Insert a bit-range")
        .value("EXTRACT", OpCode::CPUI_EXTRACT, "Extract a bit-range")
        .value("POPCOUNT", OpCode::CPUI_POPCOUNT, "Count the 1-bits")
        .value("LZCOUNT", OpCode::CPUI_LZCOUNT, "Count the leading 0-bits");

    nb::class_<PcodeOp>(m, "PcodeOp", "Low-level representation of a single P-Code operation.")
        .def_ro("opcode", &PcodeOp::m_opcode, "opcode(self) -> OpCode\nOpcode for this operation.")
        .def_ro("output", &PcodeOp::m_output, "output(self) -> Optional[Varnode]\nOutput varnode for this operation.")
        .def_ro("inputs", &PcodeOp::m_inputs, "inputs(self) -> List[Varnode]\nInput varnodes for this operation.");

    nb::class_<Translation>(m, "Translation", "P-Code translation.")
        .def_ro("ops", &Translation::m_ops, "ops(self) -> List[PcodeOp]\nThe translated sequence of P-Code ops.");

    nb::class_<DisassemblyInstruction>(m, "Instruction", "Disassembled machine code instruction.")
        .def_ro("addr", &DisassemblyInstruction::m_addr, "addr(self) -> Address\nAddress of this instruction.")
        .def_ro(
            "length", &DisassemblyInstruction::m_length, "length(self) -> int\nLength, in bytes, of this instruction.")
        .def_ro("mnem", &DisassemblyInstruction::m_mnem, "mnem(self) -> str\nMnemonic string of this instruction.")
        .def_ro("body", &DisassemblyInstruction::m_body, "body(self) -> str\nOperand string of this instruction.");

    nb::class_<Disassembly>(m, "Disassembly", "Machine Code Disassembly.")
        .def_ro("instructions",
                &Disassembly::m_instructions,
                "instructions(self) -> List[Instruction]\nThe disassembled instructions.");

    m.attr("TRANSLATE_FLAGS_BB_TERMINATING") = static_cast<int>(TranslateFlags::BB_TERMINATING);

    nb::class_<Context>(m, "Context", "Context for machine code translation and disassembly.")
        .def(nb::init<const std::string &>())
        .def(
            "disassemble",
            [](Context &t,
               nb::bytes buf,
               uint64_t base_address,
               uint64_t offset,
               uint64_t max_bytes,
               uint64_t max_instructions) {
                if (offset >= buf.size()) {
                    throw std::out_of_range("offset out of range");
                }
                uint64_t available_bytes = buf.size() - offset;
                max_bytes = max_bytes ? MIN(max_bytes, available_bytes) : available_bytes;
                return t.disassemble(buf.c_str() + offset, max_bytes, base_address, max_instructions);
            },
            "buf"_a,
            "base_address"_a = 0,
            "offset"_a = 0,
            "max_bytes"_a = 0,
            "max_instructions"_a = 0,
            nb::keep_alive<0, 1>(),
            R"(Disassemble and format machine code as assembly code.

            .. ipython::

               In [0]: import pypcode
                  ...: ctx = pypcode.Context("x86:LE:64:default")
                  ...: dx = ctx.disassemble(b"\x48\x35\x78\x56\x34\x12\xc3")
                  ...: for ins in dx.instructions:
                  ...:     print(f"{ins.addr.offset:#x}/{ins.length}: {ins.mnem} {ins.body}")
                  0x0/6: XOR RAX,0x12345678
                  0x6/1: RET

            Instructions are decoded from ``buf`` and formatted in :class:`.Instruction` s:
              * the end of the buffer is reached,
              * ``max_bytes`` or ``max_instructions`` is reached, or
              * an exception occurs.

            If an exception occurs following successful disassembly of at least one instruction, the exception is
            discarded and the successful disassembly is returned. If the exception occurs at disassembly of the first
            instruction, it will be raised. See below for possible exceptions.

            Args:
                buf (bytes): Machine code to disassemble.
                base_address (int): Base address of the code at offset being decoded, 0 by default.
                offset (int): Offset into ``bytes`` to begin disassembly, 0 by default.
                max_bytes (int): Maximum number of bytes to disassemble, or 0 for no limit (default).
                max_instructions (int): Maximum number of instructions to disassemble, or 0 for no limit (default).

            Returns:
                Disassembly: The disassembled machine code. Instructions are accessible through :attr:`.Disassembly.instructions`.

            Raises:
                BadDataError: The instruction at ``base_address`` could be decoded.
            )")
        .def(
            "getAllRegisters",
            [](Context &t) {
                map<VarnodeData, string> regmap;
                t.m_sleigh->getAllRegisters(regmap);
                return regmap;
            },
            "Get a mapping of all register locations to their corresponding names.")
        .def(
            "getRegisterName",
            [](Context &t, AddrSpace *space, uint64_t offset, uint32_t size) {
                return t.m_sleigh->getRegisterName(space, offset, size);
            },
            "space"_a,
            "offset"_a,
            "size"_a,
            R"(Get the name of a register.

            Args:
                space (AddrSpace): The address space.
                offset (int): Offset within the address space.
                size (int): Size of the register, in bytes.

            Returns:
                str: The register name, or the empty string if the register could not be identified.
            )")
        .def("reset", &Context::reset, "Reset the context.")
        .def(
            "setVariableDefault",
            [](Context &t, const std::string &name, uint32_t value) { t.m_context_db.setVariableDefault(name, value); },
            "name"_a,
            "value"_a,
            "Provide a default value for a context variable.")
        .def(
            "translate",
            [](Context &t,
               nb::bytes buf,
               uint64_t base_address,
               uint64_t offset,
               uint64_t max_bytes,
               uint64_t max_instructions,
               uint64_t flags) {
                if (offset >= buf.size()) {
                    throw std::out_of_range("offset out of range");
                }
                uint64_t available_bytes = buf.size() - offset;
                max_bytes = max_bytes ? MIN(max_bytes, available_bytes) : available_bytes;
                return t.translate(buf.c_str() + offset, max_bytes, base_address, max_instructions, flags);
            },
            "buf"_a,
            "base_address"_a = 0,
            "offset"_a = 0,
            "max_bytes"_a = 0,
            "max_instructions"_a = 0,
            "flags"_a = 0,
            nb::keep_alive<0, 1>(),
            R"(Translate machine code to P-Code.

            .. ipython::

               In [0]: import pypcode
                  ...: ctx = pypcode.Context("x86:LE:64:default")
                  ...: tx = ctx.translate(b"\x48\x35\x78\x56\x34\x12\xc3")  # xor rax, 0x12345678; ret
                  ...: print(tx)
               IMARK ram[0:6]
               CF = 0x0
               OF = 0x0
               RAX = RAX ^ 0x12345678
               SF = RAX s< 0x0
               ZF = RAX == 0x0
               unique[28080:8] = RAX & 0xff
               unique[28100:1] = popcount(unique[28080:8])
               unique[28180:1] = unique[28100:1] & 0x1
               PF = unique[28180:1] == 0x0
               IMARK ram[6:1]
               RIP = *[ram]RSP
               RSP = RSP + 0x8
               return RIP

            Instructions are decoded from ``buf`` and translated to a sequence of :class:`.PcodeOp` s until:
              * the end of the buffer is reached,
              * ``max_bytes`` or ``max_instructions`` is reached,
              * if the ``BB_TERMINATING`` flag is set, an instruction which performs a branch is encountered, or
              * an exception occurs.

            A :class:`.PcodeOp` with opcode :attr:`OpCode.IMARK` is used to identify machine instructions corresponding
            to a translation. :attr:`OpCode.IMARK` ops precede the corresponding P-Code translation, and will have one
            or more input :class:`.Varnode` s identifying the address and length in bytes of the source machine
            instruction(s). The number of input :class:`.Varnode` s depends on the number of instructions that were
            decoded for the translation of the particular instruction.

            On architectures with branch delay slots, the effects of the delay slot instructions will be included in the
            translation of the branch instruction. For this reason, it is possible that more instructions than
            specified in ``max_instructions`` may be translated. The :attr:`OpCode.IMARK` op identifying the branch
            instruction will contain an input :class:`.Varnode` corresponding to the branch instruction, with
            additional input :class:`.Varnode` identifying corresponding delay slot instructions.

            If an exception occurs following successful translation of at least one instruction, the exception is
            discarded and the successful translation is returned. If the exception occurs during translation of the
            first instruction, the exception will be raised. See below for possible exceptions.

            Args:
                buf (bytes): Machine code to translate.
                base_address (int): Base address of the code at offset being decoded.
                offset (int): Offset into ``bytes`` to begin translation.
                max_bytes (int): Maximum number of bytes to translate.
                max_instructions (int): Maximum number of instructions to translate.
                flags (int): Flags controlling translation. See :class:`.TranslateFlags`.

            Returns:
                Translation: The P-Code translation of the input machine code. P-Code ops are accessible through :attr:`.Translation.ops`.

            Raises:
                BadDataError: The instruction at ``base_address`` could not be decoded.
                UnimplError: The P-Code for instruction at ``base_address`` is not yet implemented.
            )");
}
