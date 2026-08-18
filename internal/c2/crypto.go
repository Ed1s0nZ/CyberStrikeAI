package c2

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// AES-256-GCM 信封：每个 Listener 独立 32 字节密钥 + 每条消息独立 12 字节 nonce。
// 协议格式（base64 文本，便于 HTTP body / SSE 直接传）：
//   base64( nonce(12) || ciphertext+tag )
// 设计要点：
//   - GCM 自带 16 字节 AEAD tag，完整性 + 机密性一次性搞定，无需额外 HMAC；
//   - nonce 由 crypto/rand 生成，96bit 在密钥不变期内重复概率极低（< 2^-32 / 4B 次）；
//   - 密钥不出服务端：listener 创建时随机生成 32 字节，编译 beacon 时硬编码进去。

// GenerateAESKey 生成随机 32 字节 AES-256 密钥并 base64 输出
func GenerateAESKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// GenerateImplantToken 生成 32 字节 token，base64 编码（implant 携带在 HTTP header 鉴权用）
func GenerateImplantToken() (string, error) {
	t := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, t); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(t), nil
}

// EncryptAESGCM 加密任意明文，返回 base64(nonce||ct)
func EncryptAESGCM(keyB64 string, plaintext []byte) (string, error) {
	key, err := decodeKey(keyB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptAESGCM 解密 base64(nonce||ct)，返回明文
func DecryptAESGCM(keyB64, encB64 string) ([]byte, error) {
	key, err := decodeKey(keyB64)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		return nil, errors.New("ciphertext base64 invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize+16 { // 至少 nonce + tag
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, errors.New("aead open failed (key mismatch or tampered)")
	}
	return pt, nil
}

// EncryptAESGCMWithAAD encrypts with additional authenticated data bound to context (e.g. session_id).
// Prevents cross-session replay: ciphertext from session A cannot be fed to session B.
func EncryptAESGCMWithAAD(keyB64 string, plaintext []byte, aad []byte) (string, error) {
	key, err := decodeKey(keyB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, plaintext, aad)
	out := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptAESGCMWithAAD decrypts with AAD verification.
func DecryptAESGCMWithAAD(keyB64, encB64 string, aad []byte) ([]byte, error) {
	key, err := decodeKey(keyB64)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		return nil, errors.New("ciphertext base64 invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize+16 {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	pt, err := gcm.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, errors.New("aead open failed (key mismatch, tampered, or AAD mismatch)")
	}
	return pt, nil
}

func decodeKey(keyB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, errors.New("key base64 invalid")
	}
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes (AES-256)")
	}
	return key, nil
}
