package project

import "cyberstrike-ai/internal/projectprompt"

// FactRecordingIncrementalRhythmMarkdown 见 projectprompt。
func FactRecordingIncrementalRhythmMarkdown(coordinator, subAgent bool) string {
	return projectprompt.FactRecordingIncrementalRhythmMarkdown(coordinator, subAgent)
}

// FactRecordingBlackboardSection 见 projectprompt。
func FactRecordingBlackboardSection(coordinatorDelegate bool) string {
	return projectprompt.FactRecordingBlackboardSection(coordinatorDelegate)
}

// FactRecordingSubAgentSection 见 projectprompt。
func FactRecordingSubAgentSection() string {
	return projectprompt.FactRecordingSubAgentSection()
}

// FactRecordingBlackboardSectionMarkdown 见 projectprompt。
func FactRecordingBlackboardSectionMarkdown(coordinatorDelegate bool) string {
	return projectprompt.FactRecordingBlackboardSectionMarkdown(coordinatorDelegate)
}
