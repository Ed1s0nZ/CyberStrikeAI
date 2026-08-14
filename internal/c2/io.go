package c2

import (
	"encoding/base64"
	"os"
)

// 这些薄封装存在的目的：
//   - 让 manager.go / handler 中的逻辑更直观，避免反复 import os；
//   - 便于将来用接口抽象（譬如改成 internal/storage 的实现）做单元测试。

func osMkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func osWriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
