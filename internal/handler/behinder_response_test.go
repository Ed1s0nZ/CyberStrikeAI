package handler

import (
	"encoding/base64"
	"testing"

	"go.uber.org/zap"
)

func TestBehinderParseResponseJSPUsesRawCiphertext(t *testing.T) {
	password := "rebeyond"
	proto := NewBehinderProtocol(password, "jsp")
	plain := []byte(`{"msg":"` + base64.StdEncoding.EncodeToString([]byte("cyberstrike")) + `","status":"` + base64.StdEncoding.EncodeToString([]byte("success")) + `"}`)
	ciphertext, err := proto.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt response fixture: %v", err)
	}
	ciphertext = append(ciphertext, make([]byte, proto.getMagicNum())...)

	got, err := NewBehinderHandler(zap.NewNop()).parseResponse(ciphertext, password, "jsp")
	if err != nil {
		t.Fatalf("parseResponse returned error: %v", err)
	}
	if got != "cyberstrike" {
		t.Fatalf("parseResponse = %q, want cyberstrike", got)
	}
}

func TestBehinderParseResponseJSPUsesBase64CiphertextWithoutMagic(t *testing.T) {
	password := "rebeyond"
	proto := NewBehinderProtocol(password, "jsp")
	plain := []byte(`{"msg":"` + base64.StdEncoding.EncodeToString([]byte("base64-no-magic")) + `","status":"` + base64.StdEncoding.EncodeToString([]byte("success")) + `"}`)
	ciphertext, err := proto.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt response fixture: %v", err)
	}
	body := []byte(base64.StdEncoding.EncodeToString(ciphertext))

	got, err := NewBehinderHandler(zap.NewNop()).parseResponse(body, password, "jsp")
	if err != nil {
		t.Fatalf("parseResponse returned error: %v", err)
	}
	if got != "base64-no-magic" {
		t.Fatalf("parseResponse = %q, want base64-no-magic", got)
	}
}

func TestBehinderParseResponseJSPUsesBase64CiphertextWithMagic(t *testing.T) {
	password := "rebeyond"
	proto := NewBehinderProtocol(password, "jsp")
	plain := []byte(`{"msg":"` + base64.StdEncoding.EncodeToString([]byte("base64-with-magic")) + `","status":"` + base64.StdEncoding.EncodeToString([]byte("success")) + `"}`)
	ciphertext, err := proto.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt response fixture: %v", err)
	}
	ciphertext = append(ciphertext, make([]byte, proto.getMagicNum())...)
	body := []byte(base64.StdEncoding.EncodeToString(ciphertext))

	got, err := NewBehinderHandler(zap.NewNop()).parseResponse(body, password, "jsp")
	if err != nil {
		t.Fatalf("parseResponse returned error: %v", err)
	}
	if got != "base64-with-magic" {
		t.Fatalf("parseResponse = %q, want base64-with-magic", got)
	}
}
