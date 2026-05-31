package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
)

// BehinderEncryptType 冰蝎加密类型
type BehinderEncryptType int

const (
	BehinderEncryptAES BehinderEncryptType = iota // AES 加密 (JSP/PHP/OpenSSL/ASPX)
	BehinderEncryptXOR                            // XOR 加密 (ASP/PHP无OpenSSL)
)

// BehinderProtocol 冰蝎协议实现
type BehinderProtocol struct {
	Key         string              // 16字节密钥 (MD5(password)[0:16])
	ShellType   string              // php/jsp/aspx/asp
	EncryptType BehinderEncryptType // 加密方式
}

// NewBehinderProtocol 创建冰蝎协议实例
// password: 用户密码 (默认 "rebeyond")
// shellType: Shell类型
func NewBehinderProtocol(password, shellType string) *BehinderProtocol {
	if password == "" {
		password = "rebeyond"
	}
	key := behinderDeriveKey(password)

	return &BehinderProtocol{
		Key:         key,
		ShellType:   shellType,
		EncryptType: behinderGetEncryptType(shellType),
	}
}

// behinderDeriveKey 密钥派生: MD5(password)[0:16]
func behinderDeriveKey(password string) string {
	h := md5.Sum([]byte(password))
	fullHex := hex.EncodeToString(h[:])
	return fullHex[:16]
}

// behinderGetEncryptType 根据Shell类型获取加密方式
func behinderGetEncryptType(shellType string) BehinderEncryptType {
	switch shellType {
	case "php":
		return BehinderEncryptAES // PHP with OpenSSL uses AES-CBC
	case "jsp":
		return BehinderEncryptAES // JSP uses AES-ECB
	case "aspx":
		return BehinderEncryptAES // ASPX uses AES-CBC
	case "asp":
		return BehinderEncryptXOR // ASP uses XOR
	default:
		return BehinderEncryptAES
	}
}

// Encrypt 加密命令
// plaintext: 原始命令 (格式: "func|params")
// 返回: 加密后的字节 (已Base64编码，可直接发送)
func (bp *BehinderProtocol) Encrypt(plaintext []byte) ([]byte, error) {
	var ciphertext []byte
	var err error

	switch bp.ShellType {
	case "jsp":
		ciphertext, err = bp.encryptECB(plaintext)
	case "php":
		ciphertext, err = bp.encryptCBC(plaintext, make([]byte, 16)) // IV = zeros
		if err == nil {
			ciphertext = []byte(base64.StdEncoding.EncodeToString(ciphertext))
		}
	case "aspx":
		ciphertext, err = bp.encryptCBC(plaintext, []byte(bp.Key)) // IV = key
	case "asp":
		ciphertext, err = bp.encryptXOR(plaintext)
	default:
		ciphertext, err = bp.encryptECB(plaintext)
	}

	if err != nil {
		return nil, fmt.Errorf("behinder encrypt failed: %w", err)
	}

	// ASPX 不需要额外 Base64，其他需要
	if bp.ShellType == "aspx" {
		return ciphertext, nil
	}
	return ciphertext, nil
}

// Decrypt 解密响应
// ciphertext: 服务端返回的加密数据
// 返回: 解密后的明文
func (bp *BehinderProtocol) Decrypt(ciphertext []byte) ([]byte, error) {
	var plaintext []byte
	var err error

	switch bp.ShellType {
	case "jsp":
		// JSP 需要去掉尾部 magicNum 字节
		magicNum := bp.getMagicNum()
		if len(ciphertext) > magicNum {
			ciphertext = ciphertext[:len(ciphertext)-magicNum]
		}
		plaintext, err = bp.decryptECB(ciphertext)
	case "php":
		// PHP 响应是 Base64 编码的
		decoded, decodeErr := base64.StdEncoding.DecodeString(string(ciphertext))
		if decodeErr != nil {
			return nil, fmt.Errorf("behinder php base64 decode failed: %w", decodeErr)
		}
		plaintext, err = bp.decryptCBC(decoded, make([]byte, 16))
	case "aspx":
		plaintext, err = bp.decryptCBC(ciphertext, []byte(bp.Key))
	case "asp":
		plaintext, err = bp.decryptXOR(ciphertext)
	default:
		magicNum := bp.getMagicNum()
		if len(ciphertext) > magicNum {
			ciphertext = ciphertext[:len(ciphertext)-magicNum]
		}
		plaintext, err = bp.decryptECB(ciphertext)
	}

	if err != nil {
		return nil, fmt.Errorf("behinder decrypt failed: %w", err)
	}
	return plaintext, nil
}

// getMagicNum 获取 JSP 响应的 magicNum
// magicNum = parseInt(key[0:2], 16) % 16
func (bp *BehinderProtocol) getMagicNum() int {
	if len(bp.Key) < 2 {
		return 0
	}
	val, err := strconv.ParseInt(bp.Key[:2], 16, 64)
	if err != nil {
		return 0
	}
	return int(val % 16)
}

// encryptECB AES-128-ECB 加密
func (bp *BehinderProtocol) encryptECB(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(bp.Key))
	if err != nil {
		return nil, err
	}

	// PKCS5Padding (等同于 PKCS7 for block size 16)
	plaintext = pkcs5Pad(plaintext, block.BlockSize())

	ciphertext := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], plaintext[i:i+block.BlockSize()])
	}
	return ciphertext, nil
}

// decryptECB AES-128-ECB 解密
func (bp *BehinderProtocol) decryptECB(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(bp.Key))
	if err != nil {
		return nil, err
	}

	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += block.BlockSize() {
		block.Decrypt(plaintext[i:i+block.BlockSize()], ciphertext[i:i+block.BlockSize()])
	}

	// Remove PKCS5 padding
	plaintext = pkcs5Unpad(plaintext)
	return plaintext, nil
}

// encryptCBC AES-128-CBC 加密
func (bp *BehinderProtocol) encryptCBC(plaintext, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(bp.Key))
	if err != nil {
		return nil, err
	}

	// PKCS7 padding
	plaintext = pkcs7Pad(plaintext, block.BlockSize())

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)
	return ciphertext, nil
}

// decryptCBC AES-128-CBC 解密
func (bp *BehinderProtocol) decryptCBC(ciphertext, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(bp.Key))
	if err != nil {
		return nil, err
	}

	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	plaintext = pkcs7Unpad(plaintext)
	return plaintext, nil
}

// encryptXOR XOR 加密 (ASP)
// byte[i] ^= key[(i+1) & 0xF]
func (bp *BehinderProtocol) encryptXOR(plaintext []byte) ([]byte, error) {
	keyBytes := []byte(bp.Key)
	result := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i++ {
		result[i] = plaintext[i] ^ keyBytes[(i+1)&0xF]
	}
	return result, nil
}

// decryptXOR XOR 解密 (ASP)
func (bp *BehinderProtocol) decryptXOR(ciphertext []byte) ([]byte, error) {
	// XOR 是对称的，加密和解密是同一个操作
	return bp.encryptXOR(ciphertext)
}

// pkcs5Pad PKCS5 填充 (block size = 16)
func pkcs5Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// pkcs5Unpad PKCS5 去填充
func pkcs5Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > 16 {
		return data
	}
	return data[:len(data)-padding]
}

// pkcs7Pad PKCS7 填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	return pkcs5Pad(data, blockSize) // 对于 AES，PKCS5 和 PKCS7 是一样的
}

// pkcs7Unpad PKCS7 去填充
func pkcs7Unpad(data []byte) []byte {
	return pkcs5Unpad(data)
}

// BuildBehinderCommand 构建冰蝎命令
// funcName: 函数名 (如 "exec", "info", "list", "download", "upload")
// params: 参数
// 返回: 原始命令字符串 (func|params)
func BuildBehinderCommand(funcName, params string) string {
	return funcName + "|" + params
}
