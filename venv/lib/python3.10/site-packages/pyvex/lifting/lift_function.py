import logging
from collections import defaultdict
from typing import DefaultDict

from pyvex import const
from pyvex.block import IRSB
from pyvex.const import vex_int_class
from pyvex.errors import LiftingException, NeedStatementsNotification, PyVEXError, SkipStatementsError
from pyvex.expr import Const
from pyvex.native import ffi
from pyvex.types import LiftSource, PyLiftSource

from .lifter import Lifter
from .post_processor import Postprocessor

log = logging.getLogger(__name__)

lifters: DefaultDict[str, list[type[Lifter]]] = defaultdict(list)
postprocessors: DefaultDict[str, list[type[Postprocessor]]] = defaultdict(list)


def lift(
    data: LiftSource,
    addr,
    arch,
    max_bytes=None,
    max_inst=None,
    bytes_offset=0,
    opt_level=1,
    traceflags=0,
    strict_block_end=True,
    inner=False,
    skip_stmts=False,
    collect_data_refs=False,
    cross_insn_opt=True,
    load_from_ro_regions=False,
    const_prop=False,
):
    """
    Recursively lifts blocks using the registered lifters and postprocessors. Tries each lifter in the order in
    which they are registered on the data to lift.

    If a lifter raises a LiftingException on the data, it is skipped.
    If it succeeds and returns a block with a jumpkind of Ijk_NoDecode, all of the lifters are tried on the rest
    of the data and if they work, their output is appended to the first block.

    :param arch:            The arch to lift the data as.
    :param addr:            The starting address of the block. Effects the IMarks.
    :param data:            The bytes to lift as either a python string of bytes or a cffi buffer object.
    :param max_bytes:       The maximum number of bytes to lift. If set to None, no byte limit is used.
    :param max_inst:        The maximum number of instructions to lift. If set to None, no instruction limit is used.
    :param bytes_offset:    The offset into `data` to start lifting at.
    :param opt_level:       The level of optimization to apply to the IR, -1 through 2. -1 is the strictest
                            unoptimized level, 0 is unoptimized but will perform some lookahead/lookbehind
                            optimizations, 1 performs constant propogation, and 2 performs loop unrolling,
                            which honestly doesn't make much sense in the context of pyvex. The default is 1.
    :param traceflags:      The libVEX traceflags, controlling VEX debug prints.

    .. note:: Explicitly specifying the number of instructions to lift (`max_inst`) may not always work
              exactly as expected. For example, on MIPS, it is meaningless to lift a branch or jump
              instruction without its delay slot. VEX attempts to Do The Right Thing by possibly decoding
              fewer instructions than requested. Specifically, this means that lifting a branch or jump
              on MIPS as a single instruction (`max_inst=1`) will result in an empty IRSB, and subsequent
              attempts to run this block will raise `SimIRSBError('Empty IRSB passed to SimIRSB.')`.

    .. note:: If no instruction and byte limit is used, pyvex will continue lifting the block until the block
              ends properly or until it runs out of data to lift.
    """
    if max_bytes is not None and max_bytes <= 0:
        raise PyVEXError("Cannot lift block with no data (max_bytes <= 0)")

    if not data:
        raise PyVEXError("Cannot lift block with no data (data is empty)")

    if isinstance(data, str):
        raise TypeError("Cannot pass unicode string as data to lifter")

    py_data: PyLiftSource | None
    if isinstance(data, (bytes, bytearray, memoryview)):
        py_data = data
        c_data = None
    else:
        if max_bytes is None:
            raise PyVEXError("Cannot lift block with ffi pointer and no size (max_bytes is None)")
        c_data = data
        py_data = None

    allow_arch_optimizations = True
    # In order to attempt to preserve the property that
    # VEX lifts the same bytes to the same IR at all times when optimizations are disabled
    # we hack off all of VEX's non-IROpt optimizations when opt_level == -1.
    # This is intended to enable comparisons of the lifted IR between code that happens to be
    # found in different contexts.
    if opt_level < 0:
        allow_arch_optimizations = False
        opt_level = 0

    for lifter in lifters[arch.name]:
        try:
            u_data: LiftSource = data
            if lifter.REQUIRE_DATA_C:
                if c_data is None:
                    assert py_data is not None
                    if isinstance(py_data, (bytearray, memoryview)):
                        u_data = ffi.from_buffer(ffi.BVoidP, py_data)
                    else:
                        u_data = ffi.from_buffer(ffi.BVoidP, py_data + b"\0" * 8)
                    max_bytes = min(len(py_data), max_bytes) if max_bytes is not None else len(py_data)
                else:
                    u_data = c_data
                skip = 0
            elif lifter.REQUIRE_DATA_PY:
                if bytes_offset and arch.name.startswith("ARM") and (addr & 1) == 1:
                    skip = bytes_offset - 1
                else:
                    skip = bytes_offset
                if py_data is None:
                    assert c_data is not None
                    if max_bytes is None:
                        log.debug("Cannot create py_data from c_data when no max length is given")
                        continue
                    u_data = ffi.buffer(c_data + skip, max_bytes)[:]
                else:
                    if max_bytes is None:
                        u_data = py_data[skip:]
                    else:
                        u_data = py_data[skip : skip + max_bytes]
            else:
                raise RuntimeError(
                    "Incorrect lifter configuration. What type of data does %s expect?" % lifter.__class__
                )

            try:
                final_irsb = lifter(arch, addr).lift(
                    u_data,
                    bytes_offset - skip,
                    max_bytes,
                    max_inst,
                    opt_level,
                    traceflags,
                    allow_arch_optimizations,
                    strict_block_end,
                    skip_stmts,
                    collect_data_refs=collect_data_refs,
                    cross_insn_opt=cross_insn_opt,
                    load_from_ro_regions=load_from_ro_regions,
                    const_prop=const_prop,
                )
            except SkipStatementsError:
                assert skip_stmts is True
                final_irsb = lifter(arch, addr).lift(
                    u_data,
                    bytes_offset - skip,
                    max_bytes,
                    max_inst,
                    opt_level,
                    traceflags,
                    allow_arch_optimizations,
                    strict_block_end,
                    skip_stmts=False,
                    collect_data_refs=collect_data_refs,
                    cross_insn_opt=cross_insn_opt,
                    load_from_ro_regions=load_from_ro_regions,
                    const_prop=const_prop,
                )
            break
        except LiftingException as ex:
            log.debug("Lifting Exception: %s", str(ex))
            continue
    else:
        final_irsb = IRSB.empty_block(
            arch,
            addr,
            size=0,
            nxt=Const(const.vex_int_class(arch.bits)(addr)),
            jumpkind="Ijk_NoDecode",
        )
        final_irsb.invalidate_direct_next()
        return final_irsb

    if final_irsb.size > 0 and final_irsb.jumpkind == "Ijk_NoDecode":
        # We have decoded a few bytes before we hit an undecodeable instruction.

        # Determine if this is an intentional NoDecode, like the ud2 instruction on AMD64
        nodecode_addr_expr = final_irsb.next
        if type(nodecode_addr_expr) is Const:
            nodecode_addr = nodecode_addr_expr.con.value
            next_irsb_start_addr = addr + final_irsb.size
            if nodecode_addr != next_irsb_start_addr:
                # The last instruction of the IRSB has a non-zero length. This is an intentional NoDecode.
                # The very last instruction has been decoded
                final_irsb.jumpkind = "Ijk_NoDecode"
                final_irsb.next = final_irsb.next
                final_irsb.invalidate_direct_next()
                return final_irsb

        # Decode more bytes
        if skip_stmts:
            # When gymrat will be invoked, we will merge future basic blocks to the current basic block. In this case,
            # statements are usually required.
            # TODO: In the future, we may further optimize it to handle cases where getting statements in gymrat is not
            # TODO: required.
            return lift(
                data,
                addr,
                arch,
                max_bytes=max_bytes,
                max_inst=max_inst,
                bytes_offset=bytes_offset,
                opt_level=opt_level,
                traceflags=traceflags,
                strict_block_end=strict_block_end,
                skip_stmts=False,
                collect_data_refs=collect_data_refs,
                load_from_ro_regions=load_from_ro_regions,
                const_prop=const_prop,
            )

        next_addr = addr + final_irsb.size
        if max_bytes is not None:
            max_bytes -= final_irsb.size
        if isinstance(data, (bytes, bytearray, memoryview)):
            data_left = data[final_irsb.size :]
        else:
            data_left = data + final_irsb.size
        if max_inst is not None:
            max_inst -= final_irsb.instructions
        if (max_bytes is None or max_bytes > 0) and (max_inst is None or max_inst > 0) and data_left:
            more_irsb = lift(
                data_left,
                next_addr,
                arch,
                max_bytes=max_bytes,
                max_inst=max_inst,
                bytes_offset=bytes_offset,
                opt_level=opt_level,
                traceflags=traceflags,
                strict_block_end=strict_block_end,
                inner=True,
                skip_stmts=False,
                collect_data_refs=collect_data_refs,
                load_from_ro_regions=load_from_ro_regions,
                const_prop=const_prop,
            )
            if more_irsb.size:
                # Successfully decoded more bytes
                final_irsb.extend(more_irsb)
        elif max_bytes == 0:
            # We have no more bytes left. Mark the jumpkind of the IRSB as Ijk_Boring
            if final_irsb.size > 0 and final_irsb.jumpkind == "Ijk_NoDecode":
                final_irsb.jumpkind = "Ijk_Boring"
                final_irsb.next = Const(vex_int_class(arch.bits)(final_irsb.addr + final_irsb.size))

    if not inner:
        for postprocessor in postprocessors[arch.name]:
            try:
                postprocessor(final_irsb).postprocess()
            except NeedStatementsNotification as e:
                # The post-processor cannot work without statements. Re-lift the current block with skip_stmts=False
                if not skip_stmts:
                    # sanity check
                    # Why does the post-processor raise NeedStatementsNotification when skip_stmts is False?
                    raise TypeError(
                        "Bad post-processor %s: "
                        "NeedStatementsNotification is raised when statements are available." % postprocessor.__class__
                    ) from e

                # Re-lift the current IRSB
                return lift(
                    data,
                    addr,
                    arch,
                    max_bytes=max_bytes,
                    max_inst=max_inst,
                    bytes_offset=bytes_offset,
                    opt_level=opt_level,
                    traceflags=traceflags,
                    strict_block_end=strict_block_end,
                    inner=inner,
                    skip_stmts=False,
                    collect_data_refs=collect_data_refs,
                    load_from_ro_regions=load_from_ro_regions,
                    const_prop=const_prop,
                )
            except LiftingException:
                continue

    return final_irsb


def register(lifter, arch_name):
    """
    Registers a Lifter or Postprocessor to be used by pyvex. Lifters are are given priority based on the order
    in which they are registered. Postprocessors will be run in registration order.

    :param lifter:       The Lifter or Postprocessor to register
    :vartype lifter:     :class:`Lifter` or :class:`Postprocessor`
    """
    if issubclass(lifter, Lifter):
        log.debug("Registering lifter %s for architecture %s.", lifter.__name__, arch_name)
        lifters[arch_name].append(lifter)
    if issubclass(lifter, Postprocessor):
        log.debug("Registering postprocessor %s for architecture %s.", lifter.__name__, arch_name)
        postprocessors[arch_name].append(lifter)
