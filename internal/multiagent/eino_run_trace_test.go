package multiagent

import "testing"

func TestWithEinoRunIDProgressAddsRunIDToMapData(t *testing.T) {
	var gotType, gotMessage string
	var gotData interface{}
	progress := withEinoRunIDProgress("run-1", func(eventType, message string, data interface{}) {
		gotType = eventType
		gotMessage = message
		gotData = data
	})

	progress("progress", "hello", map[string]interface{}{"source": "eino"})

	if gotType != "progress" || gotMessage != "hello" {
		t.Fatalf("event = (%q, %q)", gotType, gotMessage)
	}
	m, ok := gotData.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T", gotData)
	}
	if m["runId"] != "run-1" || m["source"] != "eino" {
		t.Fatalf("data = %#v", m)
	}
}

func TestWithEinoRunIDProgressPreservesExistingRunID(t *testing.T) {
	var got map[string]interface{}
	progress := withEinoRunIDProgress("outer-run", func(_, _ string, data interface{}) {
		got, _ = data.(map[string]interface{})
	})

	progress("progress", "", map[string]interface{}{"runId": "inner-run"})

	if got["runId"] != "inner-run" {
		t.Fatalf("runId = %q, want inner-run", got["runId"])
	}
}
