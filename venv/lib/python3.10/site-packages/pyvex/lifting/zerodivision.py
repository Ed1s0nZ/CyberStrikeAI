import copy

from pyvex import const, expr, stmt

from .post_processor import Postprocessor


class ZeroDivisionPostProcessor(Postprocessor):
    """
    A postprocessor for adding zero-division checks to VEX.

    For "div rcx", will turn:

              00 | ------ IMark(0x8000, 3, 0) ------
              01 | t0 = GET:I64(rcx)
              02 | t1 = GET:I64(rax)
              03 | t2 = GET:I64(rdx)
              04 | t3 = 64HLto128(t2,t1)
              05 | t4 = DivModU128to64(t3,t0)
              06 | t5 = 128to64(t4)
              07 | PUT(rax) = t5
              08 | t6 = 128HIto64(t4)
              09 | PUT(rdx) = t6
              NEXT: PUT(rip) = 0x0000000000008003; Ijk_Boring

    into:

              00 | ------ IMark(0x8000, 3, 0) ------
              01 | t0 = GET:I64(rcx)
              02 | t4 = GET:I64(rax)
              03 | t5 = GET:I64(rdx)
              04 | t3 = 64HLto128(t5,t4)
              05 | t9 = CmpEQ(t0,0x0000000000000000)
              06 | if (t9) { PUT(pc) = 0x8000; Ijk_SigFPE_IntDiv }
              07 | t2 = DivModU128to64(t3,t0)
              08 | t6 = 128to64(t2)
              09 | PUT(rax) = t6
              10 | t7 = 128HIto64(t2)
              11 | PUT(rdx) = t7
              NEXT: PUT(rip) = 0x0000000000008003; Ijk_Boring
    """

    def postprocess(self):
        if self.irsb.statements is None:
            # This is an optimized IRSB. We cannot really post-process it.
            return

        insertions = []
        last_ip = 0
        for i, s in enumerate(self.irsb.statements):
            if s.tag == "Ist_IMark":
                last_ip = s.addr
            if s.tag == "Ist_WrTmp" and s.data.tag == "Iex_Binop" and ("Div" in s.data.op or "Mod" in s.data.op):
                arg_size = s.data.args[1].result_size(self.irsb.tyenv)
                cmp_args = [copy.copy(s.data.args[1]), expr.Const(const.vex_int_class(arg_size)(0))]
                cmp_tmp = self.irsb.tyenv.add("Ity_I1")
                insertions.append((i, stmt.WrTmp(cmp_tmp, expr.Binop("Iop_CmpEQ%d" % arg_size, cmp_args))))
                insertions.append(
                    (
                        i,
                        stmt.Exit(
                            expr.RdTmp.get_instance(cmp_tmp),
                            const.vex_int_class(self.irsb.arch.bits)(last_ip),
                            "Ijk_SigFPE_IntDiv",
                            self.irsb.offsIP,
                        ),
                    )
                )

        for i, s in reversed(insertions):
            self.irsb.statements.insert(i, s)
