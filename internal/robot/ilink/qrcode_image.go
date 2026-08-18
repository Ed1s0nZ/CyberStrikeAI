package ilink

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

// QRCodeDataURL 将扫码内容（一般为 liteapp 链接）编码为 PNG data URL，供 Web 端展示。
// qrcode_img_content 不是图片直链，不能用作 <img src>。
func QRCodeDataURL(content string, size int) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("empty qr content")
	}
	if size <= 0 {
		size = 256
	}
	png, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
