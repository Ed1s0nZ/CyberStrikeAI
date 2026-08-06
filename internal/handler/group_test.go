package handler

import (
	"strings"
	"testing"
)

func TestValidateGroupFieldsAllowsNormalNamesAndIcons(t *testing.T) {
	name, icon, err := validateGroupFields("  日常安全巡检  ", " 📁 ")
	if err != nil {
		t.Fatalf("validateGroupFields returned error: %v", err)
	}
	if name != "日常安全巡检" {
		t.Fatalf("name = %q, want trimmed normal name", name)
	}
	if icon != "📁" {
		t.Fatalf("icon = %q, want trimmed icon", icon)
	}
}

func TestValidateGroupFieldsRejectsStoredXSSPayloads(t *testing.T) {
	tests := []struct {
		name string
		icon string
	}{
		{name: `<img src=x onerror="alert(1)">`, icon: "📁"},
		{name: "日常安全巡检", icon: `<svg onload=alert(1)>`},
		{name: "日常安全巡检`onmouseover=alert(1)", icon: "📁"},
		{name: "日常安全巡检\x00", icon: "📁"},
		{name: strings.Repeat("分", maxGroupNameRunes+1), icon: "📁"},
		{name: "日常安全巡检", icon: strings.Repeat("📁", maxGroupIconRunes+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.icon, func(t *testing.T) {
			if _, _, err := validateGroupFields(tt.name, tt.icon); err == nil {
				t.Fatal("validateGroupFields returned nil error for unsafe input")
			}
		})
	}
}
