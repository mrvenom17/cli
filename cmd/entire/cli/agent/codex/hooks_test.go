package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupTestEnv creates a temp dir, sets CWD and CODEX_HOME for test isolation.
// Cannot be parallel (uses t.Chdir and t.Setenv which are process-global).
func setupTestEnv(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	t.Setenv("CODEX_HOME", filepath.Join(tempDir, ".codex-home"))
	return tempDir
}

func TestInstallHooks_CreatesConfig(t *testing.T) {
	tempDir := setupTestEnv(t)
	codexHome := os.Getenv("CODEX_HOME")

	ag := &CodexAgent{}
	count, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)
	require.Equal(t, 3, count) // SessionStart, UserPromptSubmit, Stop

	// Verify hooks.json was created in the repo
	hooksPath := filepath.Join(tempDir, ".codex", HooksFileName)
	data, err := os.ReadFile(hooksPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "entire hooks codex session-start")
	require.Contains(t, string(data), "entire hooks codex user-prompt-submit")
	require.Contains(t, string(data), "entire hooks codex stop")

	// Verify user-level config enables codex_hooks feature and trusts this project
	configPath := filepath.Join(codexHome, configFileName)
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(configData), "codex_hooks = true")
	require.Contains(t, string(configData), "[features]")
	require.Contains(t, string(configData), `trust_level = "trusted"`)
	require.Contains(t, string(configData), tempDir) // project path in trust entry
}

func TestInstallHooks_Idempotent(t *testing.T) {
	setupTestEnv(t)

	ag := &CodexAgent{}

	// First install
	count1, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)
	require.Equal(t, 3, count1)

	// Second install should be idempotent
	count2, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)
	require.Equal(t, 0, count2)
}

func TestInstallHooks_LocalDev(t *testing.T) {
	tempDir := setupTestEnv(t)

	ag := &CodexAgent{}
	count, err := ag.InstallHooks(context.Background(), true, false)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	hooksPath := filepath.Join(tempDir, ".codex", HooksFileName)
	data, err := os.ReadFile(hooksPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "go run ${CLAUDE_PROJECT_DIR}/cmd/entire/main.go hooks codex session-start")
}

func TestInstallHooks_Force(t *testing.T) {
	setupTestEnv(t)

	ag := &CodexAgent{}

	// Install first
	_, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)

	// Force reinstall removes old and adds new
	count, err := ag.InstallHooks(context.Background(), false, true)
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func TestUninstallHooks(t *testing.T) {
	setupTestEnv(t)

	ag := &CodexAgent{}

	// Install first
	_, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)

	// Uninstall
	err = ag.UninstallHooks(context.Background())
	require.NoError(t, err)

	// Verify hooks are gone
	require.False(t, ag.AreHooksInstalled(context.Background()))
}

func TestAreHooksInstalled_NoFile(t *testing.T) {
	setupTestEnv(t)

	ag := &CodexAgent{}
	require.False(t, ag.AreHooksInstalled(context.Background()))
}

func TestAreHooksInstalled_WithHooks(t *testing.T) {
	setupTestEnv(t)

	ag := &CodexAgent{}
	_, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)

	require.True(t, ag.AreHooksInstalled(context.Background()))
}

func TestInstallHooks_PreservesExistingConfig(t *testing.T) {
	tempDir := setupTestEnv(t)

	ag := &CodexAgent{}

	// Write existing config with custom hooks
	codexDir := filepath.Join(tempDir, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o750))
	existingConfig := `{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "^Bash$",
					"hooks": [
						{"type": "command", "command": "my-custom-hook"}
					]
				}
			]
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, HooksFileName), []byte(existingConfig), 0o600))

	// Install Entire hooks
	_, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)

	// Verify custom hooks are preserved
	data, err := os.ReadFile(filepath.Join(codexDir, HooksFileName))
	require.NoError(t, err)
	require.Contains(t, string(data), "my-custom-hook")
	require.Contains(t, string(data), "entire hooks codex stop")
}

func TestInstallHooks_PreservesExistingUserConfig(t *testing.T) {
	setupTestEnv(t)
	codexHome := os.Getenv("CODEX_HOME")

	// Write existing user-level config
	require.NoError(t, os.MkdirAll(codexHome, 0o750))
	existingConfig := "model = \"gpt-4.1\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, configFileName), []byte(existingConfig), 0o600))

	ag := &CodexAgent{}
	_, err := ag.InstallHooks(context.Background(), false, false)
	require.NoError(t, err)

	// Verify existing config is preserved and feature is added
	configData, err := os.ReadFile(filepath.Join(codexHome, configFileName))
	require.NoError(t, err)
	require.Contains(t, string(configData), "model = \"gpt-4.1\"")
	require.Contains(t, string(configData), "codex_hooks = true")
}
