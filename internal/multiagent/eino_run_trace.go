package multiagent

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func newEinoRunID() string {
	return uuid.New().String()
}

func withEinoRunIDProgress(
	runID string,
	progress func(eventType, message string, data interface{}),
) func(eventType, message string, data interface{}) {
	runID = strings.TrimSpace(runID)
	if progress == nil || runID == "" {
		return progress
	}
	return func(eventType, message string, data interface{}) {
		progress(eventType, message, addEinoRunIDToProgressData(runID, data))
	}
}

func addEinoRunIDToProgressData(runID string, data interface{}) interface{} {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return data
	}
	switch v := data.(type) {
	case map[string]interface{}:
		if existing, ok := v["runId"]; !ok || strings.TrimSpace(fmt.Sprint(existing)) == "" {
			v["runId"] = runID
		}
		return v
	default:
		return data
	}
}
