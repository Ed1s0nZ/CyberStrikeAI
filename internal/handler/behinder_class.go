package handler

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// modifyClassFields rewrites Behinder Java payload classes the same way
// Behinder's Params visitor does: static String fields receive ConstantValue
// entries, and the class major version is lowered to Java 5 for older targets.
func modifyClassFields(classBytes []byte, fields map[string]string) ([]byte, error) {
	if len(classBytes) < 10 {
		return nil, fmt.Errorf("class file too short (%d bytes)", len(classBytes))
	}
	if binary.BigEndian.Uint32(classBytes[0:4]) != 0xCAFEBABE {
		return nil, fmt.Errorf("not a valid Java class file (bad magic)")
	}

	data := make([]byte, len(classBytes))
	copy(data, classBytes)

	cpCount := int(binary.BigEndian.Uint16(data[8:10]))
	entries, cpEnd := parseConstantPool(data, cpCount)

	type newCP struct {
		bytes []byte
		index int
	}
	var newEntries []newCP
	nextIdx := cpCount

	findUtf8 := func(s string) int {
		for i, e := range entries {
			if e == nil || e.tag != 1 {
				continue
			}
			if readUtf8(data, e) == s {
				return i
			}
		}
		return -1
	}

	findStringRef := func(utf8Idx int) int {
		for i, e := range entries {
			if e == nil || e.tag != 8 {
				continue
			}
			if int(binary.BigEndian.Uint16(data[e.start+1:e.start+3])) == utf8Idx {
				return i
			}
		}
		return -1
	}

	cvIdx := findUtf8("ConstantValue")
	if cvIdx < 0 {
		cvIdx = nextIdx
		newEntries = append(newEntries, newCP{bytes: encodeUtf8Constant("ConstantValue"), index: nextIdx})
		nextIdx++
	}

	type fieldMod struct {
		nameIdx      int
		stringRefIdx int
	}
	mods := make([]fieldMod, 0, len(fields))
	modByNameIdx := make(map[int]int, len(fields))
	fieldNameByIdx := make(map[int]string, len(fields))

	for fieldName, fieldValue := range fields {
		nameIdx := findUtf8(fieldName)
		if nameIdx < 0 {
			return nil, fmt.Errorf("field %q not found in constant pool", fieldName)
		}

		valUtf8Idx := findUtf8(fieldValue)
		if valUtf8Idx < 0 {
			valUtf8Idx = nextIdx
			newEntries = append(newEntries, newCP{bytes: encodeUtf8Constant(fieldValue), index: nextIdx})
			nextIdx++
		}

		strRefIdx := findStringRef(valUtf8Idx)
		if strRefIdx < 0 {
			entry := make([]byte, 3)
			entry[0] = 8
			binary.BigEndian.PutUint16(entry[1:3], uint16(valUtf8Idx))
			strRefIdx = nextIdx
			newEntries = append(newEntries, newCP{bytes: entry, index: nextIdx})
			nextIdx++
		}

		modByNameIdx[nameIdx] = len(mods)
		fieldNameByIdx[nameIdx] = fieldName
		mods = append(mods, fieldMod{nameIdx: nameIdx, stringRefIdx: strRefIdx})
	}

	header := append([]byte(nil), data[:8]...)
	if binary.BigEndian.Uint16(header[6:8]) > 49 {
		binary.BigEndian.PutUint16(header[6:8], 49)
	}

	result := append([]byte{}, header...)
	result = binary.BigEndian.AppendUint16(result, uint16(nextIdx))
	result = append(result, data[10:cpEnd]...)
	for _, ne := range newEntries {
		result = append(result, ne.bytes...)
	}

	rest := data[cpEnd:]
	pos := 0
	pos += 6 // access_flags, this_class, super_class
	ifaceCount := int(binary.BigEndian.Uint16(rest[pos : pos+2]))
	pos += 2 + ifaceCount*2
	pos += 2 // fields_count
	fieldsStart := pos

	result = append(result, rest[:fieldsStart]...)
	applied := make(map[int]bool, len(mods))

	fieldCount := int(binary.BigEndian.Uint16(rest[fieldsStart-2 : fieldsStart]))
	for i := 0; i < fieldCount; i++ {
		fieldStart := pos
		accessFlags := binary.BigEndian.Uint16(rest[pos : pos+2])
		nameIdx := int(binary.BigEndian.Uint16(rest[pos+2 : pos+4]))
		descIdx := int(binary.BigEndian.Uint16(rest[pos+4 : pos+6]))
		attrCount := int(binary.BigEndian.Uint16(rest[pos+6 : pos+8]))
		pos += 8

		attrEnd := pos
		for j := 0; j < attrCount; j++ {
			attrLen := int(binary.BigEndian.Uint32(rest[attrEnd+2 : attrEnd+6]))
			attrEnd += 6 + attrLen
		}

		modIdx, shouldModify := modByNameIdx[nameIdx]
		if !shouldModify || (accessFlags&0x0008) == 0 {
			result = append(result, rest[fieldStart:attrEnd]...)
			pos = attrEnd
			continue
		}

		result = binary.BigEndian.AppendUint16(result, accessFlags)
		result = binary.BigEndian.AppendUint16(result, uint16(nameIdx))
		result = binary.BigEndian.AppendUint16(result, uint16(descIdx))
		result = binary.BigEndian.AppendUint16(result, 1)
		result = binary.BigEndian.AppendUint16(result, uint16(cvIdx))
		result = binary.BigEndian.AppendUint32(result, 2)
		result = binary.BigEndian.AppendUint16(result, uint16(mods[modIdx].stringRefIdx))

		applied[modIdx] = true
		pos = attrEnd
	}
	result = append(result, rest[pos:]...)

	var missing []string
	for i, mod := range mods {
		if !applied[i] {
			missing = append(missing, fieldNameByIdx[mod.nameIdx])
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("static fields not found: %s", strings.Join(missing, ", "))
	}

	return result, nil
}

// cpEntry describes one constant pool entry.
type cpEntry struct {
	tag   uint8
	start int
	end   int
	index int
}

// parseConstantPool parses the constant pool from class file data.
func parseConstantPool(data []byte, cpCount int) ([]*cpEntry, int) {
	entries := make([]*cpEntry, cpCount)
	pos := 10

	for i := 1; i < cpCount; i++ {
		tag := data[pos]
		start := pos
		pos++

		switch tag {
		case 1:
			length := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2 + length
		case 3, 4:
			pos += 4
		case 5, 6:
			pos += 8
			entries[i] = &cpEntry{tag: tag, start: start, end: pos, index: i}
			i++
			continue
		case 7, 8, 16, 19, 20:
			pos += 2
		case 9, 10, 11, 12, 17, 18:
			pos += 4
		case 15:
			pos += 3
		default:
			pos += 2
		}

		entries[i] = &cpEntry{tag: tag, start: start, end: pos, index: i}
	}

	return entries, pos
}

func readUtf8(data []byte, entry *cpEntry) string {
	if entry == nil || entry.tag != 1 {
		return ""
	}
	p := entry.start + 1
	length := int(binary.BigEndian.Uint16(data[p : p+2]))
	return string(data[p+2 : p+2+length])
}

func encodeUtf8Constant(s string) []byte {
	b := []byte(s)
	result := make([]byte, 3+len(b))
	result[0] = 1
	binary.BigEndian.PutUint16(result[1:3], uint16(len(b)))
	copy(result[3:], b)
	return result
}
