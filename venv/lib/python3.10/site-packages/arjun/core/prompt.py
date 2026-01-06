import os
import tempfile


def prompt(default=None):
    """
    lets user paste input by opening a temp file in a text editor
    returns str (content of tmp file)
    """
    editor = 'nano'
    with tempfile.NamedTemporaryFile(mode='r+') as tmpfile:
        if default:
            tmpfile.write(default)
            tmpfile.flush()

        child_pid = os.fork()
        is_child = child_pid == 0

        if is_child:
            os.execvp(editor, [editor, tmpfile.name])
        else:
            os.waitpid(child_pid, 0)
            tmpfile.seek(0)
            return tmpfile.read().strip()
