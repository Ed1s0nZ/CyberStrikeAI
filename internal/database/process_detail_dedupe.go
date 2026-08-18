package database

import (
	"fmt"
	"strings"
)

// DedupeConsecutiveProcessDetails 去掉相邻且语义相同的过程详情（使用 DB 中 data 列原始 JSON 作指纹，避免 map 序列化键序不稳定）。
func DedupeConsecutiveProcessDetails(rows []ProcessDetail) []ProcessDetail {
	if len(rows) < 2 {
		return rows
	}
	out := make([]ProcessDetail, 0, len(rows))
	var lastKey string
	for _, d := range rows {
		key := processDetailRowKey(d)
		if len(out) > 0 && key != "" && key == lastKey {
			continue
		}
		out = append(out, d)
		lastKey = key
	}
	return out
}

func processDetailRowKey(d ProcessDetail) string {
	return fmt.Sprintf("%s\x00%s\x00%s", d.EventType, strings.TrimSpace(d.Message), d.Data)
}
