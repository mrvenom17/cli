package slacktriage

import "strings"

const TriageTriggerText = "triage e2e"

// NormalizeTrigger lowercases, trims, and collapses internal whitespace.
func NormalizeTrigger(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

// IsTriageTrigger reports whether text normalizes to the triage trigger phrase.
func IsTriageTrigger(text string) bool {
	return NormalizeTrigger(text) == TriageTriggerText
}
