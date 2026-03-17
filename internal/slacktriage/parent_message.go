package slacktriage

import (
	"errors"
	"fmt"
	"strings"
)

// ParentMessageMetadata captures the parsed machine-readable Slack alert metadata.
type ParentMessageMetadata struct {
	Repo         string
	Branch       string
	RunID        string
	RunURL       string
	SHA          string
	FailedAgents []string
}

// ParseParentMessageMetadata extracts the stable meta line from a Slack failure alert.
func ParseParentMessageMetadata(body string) (ParentMessageMetadata, error) {
	const metaPrefix = "meta:"

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, metaPrefix) {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(trimmed, metaPrefix)))
		values := make(map[string]string, len(fields))
		for _, field := range fields {
			key, value, ok := strings.Cut(field, "=")
			if !ok || key == "" || value == "" {
				return ParentMessageMetadata{}, fmt.Errorf("invalid meta field %q", field)
			}
			if _, exists := values[key]; exists {
				return ParentMessageMetadata{}, fmt.Errorf("duplicate meta field %q", key)
			}
			values[key] = value
		}

		metadata := ParentMessageMetadata{
			Repo:   values["repo"],
			Branch: values["branch"],
			RunID:  values["run_id"],
			RunURL: values["run_url"],
			SHA:    values["sha"],
		}
		if agents, ok := values["agents"]; ok && agents != "" {
			metadata.FailedAgents = splitAndTrimCSV(agents)
		}

		if err := metadata.validate(); err != nil {
			return ParentMessageMetadata{}, err
		}
		return metadata, nil
	}

	return ParentMessageMetadata{}, errors.New("meta line not found")
}

func (m ParentMessageMetadata) validate() error {
	switch {
	case m.Repo == "":
		return errors.New("repo is required")
	case m.Branch == "":
		return errors.New("branch is required")
	case m.RunID == "":
		return errors.New("run_id is required")
	case m.RunURL == "":
		return errors.New("run_url is required")
	case m.SHA == "":
		return errors.New("sha is required")
	case len(m.FailedAgents) == 0:
		return errors.New("failed_agents is required")
	default:
		return nil
	}
}

func splitAndTrimCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
