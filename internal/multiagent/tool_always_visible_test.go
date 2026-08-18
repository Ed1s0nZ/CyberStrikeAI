package multiagent

import "testing"

func TestToolMatchesAlwaysVisible_ExternalAliases(t *testing.T) {
	t.Parallel()
	set := expandAlwaysVisibleNameSet([]string{"zhidemai::discount_search", "read_file"})

	cases := []struct {
		runtime string
		want    bool
	}{
		{"zhidemai__discount_search", true},
		{"zhidemai::discount_search", true},
		{"read_file", true},
		{"zhidemai__product_search_pro", false},
		{"github__discount_search", false},
	}
	for _, tc := range cases {
		if got := toolMatchesAlwaysVisible(tc.runtime, set); got != tc.want {
			t.Fatalf("toolMatchesAlwaysVisible(%q) = %v, want %v", tc.runtime, got, tc.want)
		}
	}
}

func TestExpandAlwaysVisibleNameSet_LegacyShortName(t *testing.T) {
	t.Parallel()
	set := expandAlwaysVisibleNameSet([]string{"discount_search"})
	if !toolMatchesAlwaysVisible("zhidemai__discount_search", set) {
		t.Fatal("legacy short name should match external runtime tool")
	}
}
