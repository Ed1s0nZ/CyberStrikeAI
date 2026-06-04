package handler

import (
	"encoding/base64"
	"testing"
)

func TestBuildBehinderFileFieldsRenameUsesTargetPath(t *testing.T) {
	fields, err := buildBehinderFileFields("rename", "/tmp/old.txt", "", "/tmp/new.txt", 0)
	if err != nil {
		t.Fatalf("buildBehinderFileFields returned error: %v", err)
	}
	if fields["mode"] != "rename" || fields["path"] != "/tmp/old.txt" || fields["newPath"] != "/tmp/new.txt" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestBuildBehinderFileFieldsUploadChunkUsesCreateThenAppend(t *testing.T) {
	first, err := buildBehinderFileFields("upload_chunk", "/tmp/a.bin", "QUJD", "", 0)
	if err != nil {
		t.Fatalf("first chunk error: %v", err)
	}
	if first["mode"] != "create" || first["content"] != "QUJD" {
		t.Fatalf("unexpected first fields: %#v", first)
	}

	next, err := buildBehinderFileFields("upload_chunk", "/tmp/a.bin", "RA==", "", 1)
	if err != nil {
		t.Fatalf("next chunk error: %v", err)
	}
	if next["mode"] != "append" || next["content"] != "RA==" {
		t.Fatalf("unexpected next fields: %#v", next)
	}
}

func TestBuildPayloadIncludesBehinderAppendContentForTextProtocols(t *testing.T) {
	h := NewBehinderHandler(nil)
	payload, err := h.buildPayload("aspx", "FileOperation", map[string]string{
		"mode":    "append",
		"path":    "/tmp/a.bin",
		"content": "RA==",
	}, "rebeyond")
	if err != nil {
		t.Fatalf("buildPayload returned error: %v", err)
	}

	proto := NewBehinderProtocol("rebeyond", "aspx")
	plain, err := proto.Decrypt(payload)
	if err != nil {
		t.Fatalf("decrypt payload: %v", err)
	}
	if got, want := string(plain), "append|/tmp/a.bin|RA=="; got != want {
		t.Fatalf("payload mismatch: got %q want %q", got, want)
	}
}

func TestBuildBehinderFileFieldsWriteEncodesContent(t *testing.T) {
	fields, err := buildBehinderFileFields("write", "/tmp/a.txt", "hello", "", 0)
	if err != nil {
		t.Fatalf("write fields error: %v", err)
	}
	if fields["mode"] != "create" || fields["content"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("unexpected write fields: %#v", fields)
	}
}

func TestDecodeBehinderFileOutputReadDecodesInnerBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("file body"))
	if got := decodeBehinderFileOutput("read", encoded); got != "file body" {
		t.Fatalf("decoded read output = %q", got)
	}
	if got := decodeBehinderFileOutput("list", encoded); got != encoded {
		t.Fatalf("list output decoded unexpectedly: %q", got)
	}
}
