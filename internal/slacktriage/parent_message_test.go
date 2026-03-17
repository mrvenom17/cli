package slacktriage

import (
	"testing"
)

func TestParseParentMessageMetadata(t *testing.T) {
	t.Parallel()

	body := "E2E Tests Failed on `main`\n\nFailed agents: *cursor-cli*\n<https://github.com/entireio/cli/actions/runs/123|View run details>\nmeta: repo=entireio/cli branch=main run_id=123 run_url=https://github.com/entireio/cli/actions/runs/123 sha=abc123 agents=cursor-cli,copilot-cli\nCommit: <https://github.com/entireio/cli/commit/abc123|abc123> by alisha"

	got, err := ParseParentMessageMetadata(body)
	if err != nil {
		t.Fatalf("ParseParentMessageMetadata() error = %v", err)
	}

	wantAgents := []string{"cursor-cli", "copilot-cli"}
	if got.Repo != "entireio/cli" {
		t.Fatalf("Repo = %q, want %q", got.Repo, "entireio/cli")
	}
	if got.Branch != "main" {
		t.Fatalf("Branch = %q, want %q", got.Branch, "main")
	}
	if got.RunID != "123" {
		t.Fatalf("RunID = %q, want %q", got.RunID, "123")
	}
	if got.RunURL != "https://github.com/entireio/cli/actions/runs/123" {
		t.Fatalf("RunURL = %q, want %q", got.RunURL, "https://github.com/entireio/cli/actions/runs/123")
	}
	if got.SHA != "abc123" {
		t.Fatalf("SHA = %q, want %q", got.SHA, "abc123")
	}
	if len(got.FailedAgents) != len(wantAgents) {
		t.Fatalf("FailedAgents len = %d, want %d", len(got.FailedAgents), len(wantAgents))
	}
	for i := range wantAgents {
		if got.FailedAgents[i] != wantAgents[i] {
			t.Fatalf("FailedAgents[%d] = %q, want %q", i, got.FailedAgents[i], wantAgents[i])
		}
	}
}

func TestParseParentMessageMetadata_IgnoresHumanReadableBody(t *testing.T) {
	t.Parallel()

	body := "E2E Tests Failed on `main`\n\nFailed agents: *cursor-cli*\n<https://github.com/entireio/cli/actions/runs/123|View run details>\nCommit: <https://github.com/entireio/cli/commit/abc123|abc123> by alisha"

	if _, err := ParseParentMessageMetadata(body); err == nil {
		t.Fatal("ParseParentMessageMetadata() error = nil, want error")
	}
}
