package security

import "testing"

func TestMatchDangerousCommand(t *testing.T) {
	cases := []struct {
		cmd      string
		hit      bool
		desc     string
	}{
		// 应拦截
		{"rm -rf /", true, "rm -rf 根目录"},
		{"rm -rf /tmp/x", true, "rm -rf 任意目标"},
		{"rm -fr /tmp/x", true, "rm -fr 变体"},
		{"rm -r -f /tmp/x", true, "rm 分写"},
		{"sudo rm -rf /etc", true, "sudo rm -rf 系统目录"},
		{"rm -rf ~", true, "rm -rf 家目录"},
		{"rm -rf *", true, "rm -rf 通配"},
		{"del /f /s /q C:\\temp", true, "del 递归删除"},
		{"rd /s /q C:\\temp", true, "rd 递归删除"},
		{"rmdir /s C:\\temp", true, "rmdir 递归删除"},
		{"Remove-Item -Recurse C:\\data", true, "Remove-Item 递归"},
		{"Remove-Item -Force C:\\x\\a.txt", true, "Remove-Item 强制"},
		{"format C:", true, "format 磁盘"},
		{"format D: /q", true, "format 磁盘变体"},
		{"diskpart", true, "diskpart"},
		{"shutdown /s /t 0", true, "shutdown"},
		{"shutdown -h now", true, "shutdown unix"},
		{"reboot", true, "reboot"},
		{"Stop-Service spooler", true, "Stop-Service"},
		{"Stop-Process -Name msedge", true, "Stop-Process"},
		{"taskkill /f /im notepad.exe", true, "taskkill /f"},
		{"sc delete MySvc", true, "sc delete"},
		{"reg delete HKLM\\Software\\X", true, "reg delete"},
		{"git push --force origin main", true, "git 强制推送"},
		{"git push -f origin main", true, "git 强制推送短参"},
		{"rm /etc/passwd", true, "删除系统关键文件"},
		{"del C:\\Windows\\System32\\x.dll", true, "删除系统目录"},
		{"chmod -R 777 /var/www", true, "chmod -R 危险权限"},
		{"dd if=/dev/zero of=/dev/sda", true, "dd 写块设备"},

		// 不应拦截（合法/只读/单文件）
		{"rm -f /tmp/x.log", false, "单文件删除"},
		{"rm /tmp/x.log", false, "普通 rm"},
		{"del /f C:\\temp\\a.txt", false, "del 单文件"},
		{"rd C:\\temp\\a.txt", false, "rd 单文件"},
		{"dir C:\\Windows", false, "目录浏览"},
		{"ls -la /etc", false, "只读列出"},
		{"nmap -sV -p 1-65535 192.168.1.1", false, "nmap 扫描"},
		{"taskkill /im notepad.exe", false, "taskkill 无 /f"},
		{"Get-Process", false, "只读进程列表"},
		{"Get-Service spooler", false, "只读服务列表"},
		{"echo 777", false, "数字 777 不应误伤"},
		{"python3 -c 'print(1)'", false, "合法脚本"},
		{"git push origin main", false, "普通推送"},
		{"git status", false, "git 状态"},
		{"curl -k https://example.com", false, "curl 请求"},
		{"sqlmap -u http://x/ -dbs", false, "sqlmap 扫描"},
	}
	for _, c := range cases {
		reason, got := MatchDangerousCommand(c.cmd, nil)
		if got != c.hit {
			t.Errorf("[%s] cmd=%q 期望 hit=%v 实际=%v (reason=%q)", c.desc, c.cmd, c.hit, got, reason)
		}
	}
}

func TestMatchDangerousCommandCustom(t *testing.T) {
	// 自定义正则追加生效
	reason, hit := MatchDangerousCommand("my-weird-tool --nuke", []string{`\bnuke\b`})
	if !hit || reason == "" {
		t.Fatalf("自定义规则未生效: hit=%v reason=%q", hit, reason)
	}
	// 非法正则应被忽略且不误伤
	_, hit = MatchDangerousCommand("ls -la", []string{"[invalid("})
	if hit {
		t.Fatalf("非法正则不应命中")
	}
}
