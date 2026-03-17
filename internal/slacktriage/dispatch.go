package slacktriage

// DispatchPayload is the structured payload sent to GitHub repository_dispatch.
type DispatchPayload struct {
	TriggerText   string   `json:"trigger_text"`
	Repo          string   `json:"repo"`
	Branch        string   `json:"branch"`
	SHA           string   `json:"sha"`
	RunURL        string   `json:"run_url"`
	RunID         string   `json:"run_id"`
	FailedAgents  []string `json:"failed_agents"`
	SlackChannel  string   `json:"slack_channel"`
	SlackThreadTS string   `json:"slack_thread_ts"`
	SlackUser     string   `json:"slack_user"`
}

// NewDispatchPayload creates a pure data payload for the repository_dispatch bridge.
func NewDispatchPayload(meta ParentMessageMetadata, slackChannel, slackThreadTS, slackUser string) DispatchPayload {
	failedAgents := make([]string, len(meta.FailedAgents))
	copy(failedAgents, meta.FailedAgents)

	return DispatchPayload{
		TriggerText:   TriageTriggerText,
		Repo:          meta.Repo,
		Branch:        meta.Branch,
		SHA:           meta.SHA,
		RunURL:        meta.RunURL,
		RunID:         meta.RunID,
		FailedAgents:  failedAgents,
		SlackChannel:  slackChannel,
		SlackThreadTS: slackThreadTS,
		SlackUser:     slackUser,
	}
}
