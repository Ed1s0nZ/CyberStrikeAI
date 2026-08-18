package project

import "strings"

// VisionImageSectionMarker 图片分析 section 标题（与 AppendVisionImageAnalysisIfReady 注入一致）。
const VisionImageSectionMarker = "## 图片分析"

// VisionImageAnalysisSection 单/多代理共用的图片分析提示（analyze_image；上下文仅保留文字摘要）。
func VisionImageAnalysisSection() string {
	var b strings.Builder
	b.WriteString(VisionImageSectionMarker)
	b.WriteString("\n\n")
	b.WriteString("- 遇到图片文件（截图、验证码、登录页、报告配图）时，若存在工具 analyze_image，请传入服务器上的文件路径进行分析。\n")
	b.WriteString("- 不要对二进制图片使用 read_file 指望理解内容；用户消息中「📎 xxx.png: /path」即为可传给 analyze_image 的路径。\n")
	b.WriteString("- 验证码类：若已从页面或接口保存为本地图片（如 captcha.png），用 analyze_image，question 写明「只输出验证码字符」；识别失败则刷新验证码后重新保存再识；复杂滑块/行为验证码勿指望单次识图成功。\n")
	b.WriteString("- 委派子代理时，若子任务含验证码/截图识读，在 task description 中写明图片路径与期望输出格式。\n")
	return b.String()
}

// AppendVisionImageAnalysisIfReady 仅在 vision.enabled 且 model 已配置时追加图片分析提示。
func AppendVisionImageAnalysisIfReady(base string, visionReady bool) string {
	if !visionReady {
		return base
	}
	return AppendSystemPromptBlock(base, VisionImageAnalysisSection())
}
