package handler

import "testing"

func TestHITLBuiltInWhitelistExemptsWriteFile(t *testing.T) {
	h := &AgentHandler{}
	req := h.hitlRequestWithMergedConfigWhitelist(&HITLRequest{
		Enabled: true,
		Mode:    "approval",
	})

	manager := NewHITLManager(nil, nil)
	manager.ActivateConversation("conversation-1", req)

	if manager.NeedsToolApproval("conversation-1", "write_file") {
		t.Fatal("write_file should use the built-in HITL exemption")
	}
	if !manager.NeedsToolApproval("conversation-1", "exec") {
		t.Fatal("non-exempt tools should still require approval")
	}
}
