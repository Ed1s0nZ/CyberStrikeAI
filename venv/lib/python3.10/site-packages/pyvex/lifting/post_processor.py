#
# The post-processor base class
#


class Postprocessor:
    def __init__(self, irsb):
        self.irsb = irsb

    def postprocess(self):
        """
        Modify the irsb

        All of the postprocessors will be used in the order that they are registered
        """
        pass
