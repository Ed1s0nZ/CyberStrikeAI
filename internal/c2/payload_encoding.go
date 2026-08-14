package c2

import (
	"encoding/base64"
	"encoding/binary"
)

// b64StdEncode 用标准 base64 编码字节
func b64StdEncode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// utf16LEBase64 把字符串转 UTF-16LE 后再 base64，用于 PowerShell -EncodedCommand
// （Windows PowerShell 接受这种格式，避免命令行特殊字符引起转义错误）
func utf16LEBase64(s string) string {
	runes := []rune(s)
	buf := make([]byte, 0, len(runes)*2)
	for _, r := range runes {
		// 注意：>0xFFFF 的字符需要代理对，但 PowerShell 命令通常都在 BMP 内
		var enc [2]byte
		binary.LittleEndian.PutUint16(enc[:], uint16(r))
		buf = append(buf, enc[:]...)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
