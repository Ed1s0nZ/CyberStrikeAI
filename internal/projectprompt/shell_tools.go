package projectprompt

// ShellExecExecuteGuidanceSection 供单代理/多代理系统提示追加：exec 与 execute 分工（尽量短）。
func ShellExecExecuteGuidanceSection() string {
	return `Shell（exec/execute）：有专用 MCP 工具时优先专用工具；系统命令（管道、workdir、后台 &）用 exec；skills/ 内脚本（配合 read_file、skill）用 execute；多步扫描分拆调用，禁止一条 shell 串多个扫描器。长脚本、请求体或 Payload 必须先用 write_file 写入会话工作目录，再用 exec/execute 执行短命令；禁止把长内容嵌入 command。下载/临时文件须写入系统提示中的「会话工作目录」，禁止用 /tmp。`
}

// ShellExecExecuteGuidanceReconSuffix 侦察子代理可选追加（一行）。
func ShellExecExecuteGuidanceReconSuffix() string {
	return `枚举优先 subfinder、amass 等专用 MCP，勿 exec/execute 拼长链。`
}
