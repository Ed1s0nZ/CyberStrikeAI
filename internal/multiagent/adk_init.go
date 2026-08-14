package multiagent

import (
	"fmt"

	"github.com/cloudwego/eino/adk"
)

// InitADK configures global Eino ADK settings. Call once at process startup before
// any ADK middleware or agents are created.
func InitADK() error {
	if err := adk.SetLanguage(adk.LanguageChinese); err != nil {
		return fmt.Errorf("adk set language: %w", err)
	}
	return nil
}
