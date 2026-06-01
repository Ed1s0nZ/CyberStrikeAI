package handler

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestBehinderProtocolRoundTripByShellType(t *testing.T) {
	plaintext := []byte("exec|whoami && echo CyberStrikeAI")
	tests := []string{"jsp", "php", "aspx", "asp"}

	for _, shellType := range tests {
		t.Run(shellType, func(t *testing.T) {
			proto := NewBehinderProtocol("rebeyond", shellType)
			encrypted, err := proto.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt returned error: %v", err)
			}
			if bytes.Equal(encrypted, plaintext) {
				t.Fatalf("encrypted payload unexpectedly equals plaintext")
			}

			decryptInput := encrypted
			if shellType == "jsp" {
				decryptInput = append(append([]byte{}, encrypted...), bytes.Repeat([]byte{0}, proto.getMagicNum())...)
			}
			decrypted, err := proto.Decrypt(decryptInput)
			if err != nil {
				t.Fatalf("Decrypt returned error: %v", err)
			}
			if !bytes.Equal(decrypted, plaintext) {
				t.Fatalf("round trip mismatch: got %q want %q", decrypted, plaintext)
			}
			if shellType == "php" {
				if _, err := base64.StdEncoding.DecodeString(string(encrypted)); err != nil {
					t.Fatalf("PHP encrypted payload should be base64 encoded: %v", err)
				}
			}
		})
	}
}

func TestBuildBehinderFileFieldsUploadChunkUsesChunkIndex(t *testing.T) {
	first, err := buildBehinderFileFields("upload_chunk", "/tmp/a.txt", "YWJj", "", 0)
	if err != nil {
		t.Fatalf("first chunk returned error: %v", err)
	}
	if first["mode"] != "create" {
		t.Fatalf("first chunk mode mismatch: got %q want create", first["mode"])
	}

	next, err := buildBehinderFileFields("upload_chunk", "/tmp/a.txt", "ZGVm", "", 1)
	if err != nil {
		t.Fatalf("next chunk returned error: %v", err)
	}
	if next["mode"] != "append" {
		t.Fatalf("next chunk mode mismatch: got %q want append", next["mode"])
	}
}

func TestBuildBehinderCommand(t *testing.T) {
	got := BuildBehinderCommand("exec", "whoami")
	if got != "exec|whoami" {
		t.Fatalf("command mismatch: got %q want %q", got, "exec|whoami")
	}
}
