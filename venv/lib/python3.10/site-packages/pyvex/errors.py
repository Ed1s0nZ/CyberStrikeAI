class PyVEXError(Exception):
    pass


class SkipStatementsError(PyVEXError):
    pass


#
# Exceptions and notifications that post-processors can raise
#


class LiftingException(Exception):
    pass


class NeedStatementsNotification(LiftingException):
    """
    A post-processor may raise a NeedStatementsNotification if it needs to work with statements, but the current IRSB
    is generated without any statement available (skip_stmts=True). The lifter will re-lift the current block with
    skip_stmts=False upon catching a NeedStatementsNotification, and re-run the post-processors.

    It's worth noting that if a post-processor always raises this notification for every basic block without statements,
    it will essentially disable the skipping statement optimization, and it is bad for performance (especially for
    CFGFast, which heavily relies on this optimization). Post-processor authors are encouraged to at least filter the
    IRSBs based on available properties (jumpkind, next, etc.). If a post-processor must work with statements for the
    majority of IRSBs, the author should implement it in PyVEX in C for the sake of a better performance.
    """

    pass
