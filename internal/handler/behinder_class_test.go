package handler

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"
)

func TestBehinderPayloadSourcesDoNotIncludeClassFiles(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("behinder_payloads", "java", "*.class"))
	if err != nil {
		t.Fatalf("glob class files: %v", err)
	}
	if len(matches) > 0 {
		t.Fatalf("precompiled class files must not be committed: %v", matches)
	}
}

func TestModifyClassFieldsInjectsMultipleStaticStrings(t *testing.T) {
	classBytes := mustReadBehinderPayloadClass(t, "FileOperation.class")
	modified, err := modifyClassFields(classBytes, map[string]string{
		"mode":    "create",
		"path":    "/tmp/cyberstrike.txt",
		"content": "Y3liZXJzdHJpa2U=",
		"charset": "UTF-8",
		"newPath": "/tmp/cyberstrike-renamed.txt",
	})
	if err != nil {
		t.Fatalf("modifyClassFields returned error: %v", err)
	}
	if got := binary.BigEndian.Uint16(modified[6:8]); got != 49 {
		t.Fatalf("major version = %d, want 49", got)
	}

	values := staticStringConstantValues(t, modified)
	want := map[string]string{
		"mode":    "create",
		"path":    "/tmp/cyberstrike.txt",
		"content": "Y3liZXJzdHJpa2U=",
		"charset": "UTF-8",
		"newPath": "/tmp/cyberstrike-renamed.txt",
	}
	for field, value := range want {
		if values[field] != value {
			t.Fatalf("field %s = %q, want %q", field, values[field], value)
		}
	}
}

func TestModifyClassFieldsSupportsCmdPath(t *testing.T) {
	classBytes := mustReadBehinderPayloadClass(t, "Cmd.class")
	modified, err := modifyClassFields(classBytes, map[string]string{
		"cmd":  "whoami",
		"path": "/tmp",
	})
	if err != nil {
		t.Fatalf("modifyClassFields returned error: %v", err)
	}
	values := staticStringConstantValues(t, modified)
	if values["cmd"] != "whoami" || values["path"] != "/tmp" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func mustReadBehinderPayloadClass(t *testing.T, name string) []byte {
	t.Helper()
	data, err := readBehinderPayloadClass(filepath.Join("behinder_payloads", "java"), strings.TrimSuffix(name, ".class"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func staticStringConstantValues(t *testing.T, classBytes []byte) map[string]string {
	t.Helper()
	cpCount := int(binary.BigEndian.Uint16(classBytes[8:10]))
	entries, cpEnd := parseConstantPool(classBytes, cpCount)
	utf8 := func(idx int) string {
		if idx <= 0 || idx >= len(entries) {
			return ""
		}
		return readUtf8(classBytes, entries[idx])
	}
	stringConst := func(idx int) string {
		if idx <= 0 || idx >= len(entries) || entries[idx] == nil || entries[idx].tag != 8 {
			return ""
		}
		ref := int(binary.BigEndian.Uint16(classBytes[entries[idx].start+1 : entries[idx].start+3]))
		return utf8(ref)
	}

	rest := classBytes[cpEnd:]
	pos := 0
	pos += 6
	ifaceCount := int(binary.BigEndian.Uint16(rest[pos : pos+2]))
	pos += 2 + ifaceCount*2
	fieldCount := int(binary.BigEndian.Uint16(rest[pos : pos+2]))
	pos += 2

	values := map[string]string{}
	for i := 0; i < fieldCount; i++ {
		accessFlags := binary.BigEndian.Uint16(rest[pos : pos+2])
		nameIdx := int(binary.BigEndian.Uint16(rest[pos+2 : pos+4]))
		descIdx := int(binary.BigEndian.Uint16(rest[pos+4 : pos+6]))
		attrCount := int(binary.BigEndian.Uint16(rest[pos+6 : pos+8]))
		pos += 8
		fieldName := utf8(nameIdx)
		isStaticString := accessFlags&0x0008 != 0 && utf8(descIdx) == "Ljava/lang/String;"
		for j := 0; j < attrCount; j++ {
			attrNameIdx := int(binary.BigEndian.Uint16(rest[pos : pos+2]))
			attrLen := int(binary.BigEndian.Uint32(rest[pos+2 : pos+6]))
			if isStaticString && utf8(attrNameIdx) == "ConstantValue" && attrLen == 2 {
				constIdx := int(binary.BigEndian.Uint16(rest[pos+6 : pos+8]))
				values[fieldName] = stringConst(constIdx)
			}
			pos += 6 + attrLen
		}
	}
	return values
}
