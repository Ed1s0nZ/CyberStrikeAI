package security

import "os/exec"

// PrepareShellCmdSession prepares a command so TerminateCmdTree can stop its
// descendant processes as a group on platforms that support it.
func PrepareShellCmdSession(cmd *exec.Cmd) error {
	return prepareShellCmdSession(cmd)
}

// TerminateCmdTree best-effort terminates cmd and child processes spawned by it.
func TerminateCmdTree(cmd *exec.Cmd) {
	terminateCmdTree(cmd)
}
