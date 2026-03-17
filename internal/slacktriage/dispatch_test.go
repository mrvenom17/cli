package slacktriage

import "testing"

func TestNewDispatchPayload(t *testing.T) {
	t.Parallel()

	meta := ParentMessageMetadata{
		Repo:         "entireio/cli",
		Branch:       "main",
		RunID:        "123",
		RunURL:       "https://github.com/entireio/cli/actions/runs/123",
		SHA:          "abc123",
		FailedAgents: []string{"cursor-cli", "copilot-cli"},
	}

	got := NewDispatchPayload(meta, "C123", "1742230000.123456", "U456")

	if got.TriggerText != TriageTriggerText {
		t.Fatalf("TriggerText = %q, want %q", got.TriggerText, TriageTriggerText)
	}
	if got.Repo != meta.Repo || got.Branch != meta.Branch || got.RunID != meta.RunID || got.RunURL != meta.RunURL || got.SHA != meta.SHA {
		t.Fatalf("payload metadata mismatch: got %+v want %+v", got, meta)
	}
	if got.SlackChannel != "C123" {
		t.Fatalf("SlackChannel = %q, want %q", got.SlackChannel, "C123")
	}
	if got.SlackThreadTS != "1742230000.123456" {
		t.Fatalf("SlackThreadTS = %q, want %q", got.SlackThreadTS, "1742230000.123456")
	}
	if got.SlackUser != "U456" {
		t.Fatalf("SlackUser = %q, want %q", got.SlackUser, "U456")
	}
	if len(got.FailedAgents) != len(meta.FailedAgents) {
		t.Fatalf("FailedAgents len = %d, want %d", len(got.FailedAgents), len(meta.FailedAgents))
	}
	for i := range meta.FailedAgents {
		if got.FailedAgents[i] != meta.FailedAgents[i] {
			t.Fatalf("FailedAgents[%d] = %q, want %q", i, got.FailedAgents[i], meta.FailedAgents[i])
		}
	}
}
