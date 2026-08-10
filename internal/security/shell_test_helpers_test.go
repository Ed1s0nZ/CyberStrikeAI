package security

import "runtime"

// shellExecPair 返回当前平台可用的 shell 与等价命令：
// Unix 用 sh -c 执行 shCmd；Windows 用 powershell -Command 执行 psCmd。
// 供集成测试在两个平台都能真实执行（Windows 默认无 sh；生产代码 getSystemShell
// 在 Windows 返回 powershell.exe -Command，这里与生产行为保持一致）。
func shellExecPair(shCmd, psCmd string) (shell string, arg string, cmd string) {
	if runtime.GOOS == "windows" {
		return "powershell", "-Command", psCmd
	}
	return "sh", "-c", shCmd
}
