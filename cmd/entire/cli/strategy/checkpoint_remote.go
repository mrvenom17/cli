package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/entireio/cli/cmd/entire/cli/logging"
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/settings"

	"github.com/go-git/go-git/v6/plumbing"
)

// checkpointRemoteName is the git remote name used for the dedicated checkpoint remote.
const checkpointRemoteName = "entire-checkpoints"

// checkpointRemoteFetchTimeout is the timeout for fetching branches from the checkpoint URL.
const checkpointRemoteFetchTimeout = 30 * time.Second

// Git remote protocol identifiers.
const (
	protocolSSH   = "ssh"
	protocolHTTPS = "https"
)

// pushSettings holds the resolved push configuration from a single settings load.
type pushSettings struct {
	// remote is the git remote name to use for checkpoint branches.
	remote string
	// pushDisabled is true if push_sessions is explicitly set to false.
	pushDisabled bool
}

// resolvePushSettings loads settings once and returns the resolved remote and push config.
// If a checkpoint_remote URL is configured:
//   - Ensures a git remote named "entire-checkpoints" is configured with that URL
//   - If a checkpoint branch doesn't exist locally, attempts to fetch it from the remote
//   - Returns "entire-checkpoints" as the remote name
//
// The push itself handles failures gracefully (doPushBranch warns and continues),
// so no reachability check is needed here. This avoids adding latency on every push
// when the remote is temporarily unreachable.
func resolvePushSettings(ctx context.Context, defaultRemote string) pushSettings {
	s, err := settings.Load(ctx)
	if err != nil {
		return pushSettings{remote: defaultRemote}
	}

	ps := pushSettings{
		remote:       defaultRemote,
		pushDisabled: s.IsPushSessionsDisabled(),
	}

	if ps.pushDisabled {
		return ps
	}

	remoteURL := s.GetCheckpointRemote()
	if remoteURL == "" {
		return ps
	}

	if err := validateRemoteURL(remoteURL); err != nil {
		logging.Warn(ctx, "checkpoint-remote: invalid URL in settings",
			slog.String("url", remoteURL),
			slog.String("error", err.Error()),
		)
		return ps
	}

	// Ensure the git remote exists with the correct URL (local operation, no network)
	if err := ensureGitRemote(ctx, checkpointRemoteName, remoteURL); err != nil {
		logging.Warn(ctx, "checkpoint-remote: failed to configure git remote",
			slog.String("url", remoteURL),
			slog.String("error", err.Error()),
		)
		return ps
	}

	ps.remote = checkpointRemoteName

	// If checkpoint branches don't exist locally, try to fetch them from the remote.
	// This is a one-time operation per branch — once the branch exists locally,
	// subsequent pushes skip the fetch entirely.
	for _, branchName := range []string{paths.MetadataBranchName, paths.TrailsBranchName} {
		if err := fetchBranchIfMissing(ctx, checkpointRemoteName, branchName); err != nil {
			logging.Warn(ctx, "checkpoint-remote: failed to fetch branch",
				slog.String("branch", branchName),
				slog.String("error", err.Error()),
			)
		}
	}

	return ps
}

// validateRemoteURL performs basic validation on a git remote URL.
// Rejects obviously malformed values that would produce confusing git errors.
func validateRemoteURL(url string) error {
	if strings.ContainsAny(url, " \t\n\r") {
		return fmt.Errorf("URL contains whitespace")
	}
	if strings.ContainsAny(url, ";|&$`\\") {
		return fmt.Errorf("URL contains invalid characters")
	}
	return nil
}

// ensureGitRemote creates or updates a git remote to point to the given URL.
// This is a local-only operation (no network calls).
func ensureGitRemote(ctx context.Context, name, url string) error {
	// Check if remote already exists and get its current URL
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", name)
	output, err := cmd.Output()
	if err != nil {
		// Remote doesn't exist, create it
		addCmd := exec.CommandContext(ctx, "git", "remote", "add", name, url)
		if addErr := addCmd.Run(); addErr != nil {
			return fmt.Errorf("failed to add remote: %w", addErr)
		}
		return nil
	}

	// Remote exists, check if URL matches
	currentURL := strings.TrimSpace(string(output))
	if currentURL == url {
		return nil
	}

	// URL differs, update it
	setCmd := exec.CommandContext(ctx, "git", "remote", "set-url", name, url)
	if setErr := setCmd.Run(); setErr != nil {
		return fmt.Errorf("failed to update remote URL: %w", setErr)
	}

	return nil
}

// fetchBranchIfMissing fetches a branch from the remote only if it doesn't exist locally.
// This avoids network calls on every push - once the branch exists locally, this is a no-op.
// If the fetch fails (remote unreachable, branch doesn't exist on remote), the error is
// returned but the caller should treat it as non-fatal: the push will handle it.
func fetchBranchIfMissing(ctx context.Context, remote, branchName string) error {
	repo, err := OpenRepository(ctx)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Check if branch already exists locally - if so, nothing to do
	branchRef := plumbing.NewBranchReferenceName(branchName)
	if _, err := repo.Reference(branchRef, true); err == nil {
		return nil // Branch exists locally, skip fetch
	}

	// Branch doesn't exist locally - try to fetch it from the remote
	fetchCtx, cancel := context.WithTimeout(ctx, checkpointRemoteFetchTimeout)
	defer cancel()

	refSpec := fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", branchName, remote, branchName)
	fetchCmd := exec.CommandContext(fetchCtx, "git", "fetch", "--no-tags", remote, refSpec)
	fetchCmd.Stdin = nil
	fetchCmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // Prevent interactive auth prompts
	)
	if err := fetchCmd.Run(); err != nil {
		// Fetch failed - remote may be unreachable or branch doesn't exist there yet.
		// Not fatal: push will create it on the remote when it succeeds.
		return nil
	}

	// Fetch succeeded - create local branch from the remote ref
	remoteRefName := plumbing.NewRemoteReferenceName(remote, branchName)
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		// Fetch succeeded but remote ref not found - branch may not exist on remote
		return nil
	}

	newRef := plumbing.NewHashReference(branchRef, remoteRef.Hash())
	if err := repo.Storer.SetReference(newRef); err != nil {
		return fmt.Errorf("failed to create local branch from remote: %w", err)
	}

	logging.Info(ctx, "checkpoint-remote: fetched branch from remote",
		slog.String("branch", branchName),
		slog.String("remote", remote),
	)
	return nil
}

// gitRemoteInfo holds parsed components of a git remote URL.
type gitRemoteInfo struct {
	protocol string // "ssh" or "https"
	host     string // e.g., "github.com"
	owner    string // e.g., "org"
	repo     string // e.g., "my-repo" (without .git)
}

// parseGitRemoteURL parses a git remote URL into its components.
// Supports:
//   - SSH SCP format: git@github.com:org/repo.git
//   - HTTPS format: https://github.com/org/repo.git
//   - SSH protocol format: ssh://git@github.com/org/repo.git
func parseGitRemoteURL(rawURL string) (*gitRemoteInfo, error) {
	rawURL = strings.TrimSpace(rawURL)

	// SSH SCP format: git@github.com:org/repo.git
	if strings.Contains(rawURL, ":") && !strings.Contains(rawURL, "://") {
		// Split on the first ":"
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid SSH URL: %s", redactURL(rawURL))
		}
		hostPart := parts[0] // e.g., "git@github.com"
		pathPart := parts[1] // e.g., "org/repo.git"

		host := hostPart
		if idx := strings.Index(host, "@"); idx >= 0 {
			host = host[idx+1:]
		}

		owner, repo, err := splitOwnerRepo(pathPart)
		if err != nil {
			return nil, err
		}

		return &gitRemoteInfo{protocol: protocolSSH, host: host, owner: owner, repo: repo}, nil
	}

	// URL format: https://github.com/org/repo.git or ssh://git@github.com/org/repo.git
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", redactURL(rawURL))
	}

	protocol := u.Scheme
	if protocol == "" {
		return nil, fmt.Errorf("no protocol in URL: %s", redactURL(rawURL))
	}
	host := u.Hostname()

	// Path is like /org/repo.git — trim leading slash
	pathPart := strings.TrimPrefix(u.Path, "/")
	owner, repo, err := splitOwnerRepo(pathPart)
	if err != nil {
		return nil, err
	}

	return &gitRemoteInfo{protocol: protocol, host: host, owner: owner, repo: repo}, nil
}

// splitOwnerRepo splits "org/repo.git" into owner and repo (without .git suffix).
func splitOwnerRepo(path string) (string, string, error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from path: %s", path)
	}
	return parts[0], parts[1], nil
}

// deriveCheckpointURL constructs a checkpoint remote URL using the same protocol
// as the push remote. For example, if push remote uses SSH, the checkpoint URL
// will also use SSH. The checkpointRepo is in "owner/repo" format.
func deriveCheckpointURL(pushRemoteURL string, checkpointRepo string) (string, error) {
	info, err := parseGitRemoteURL(pushRemoteURL)
	if err != nil {
		return "", fmt.Errorf("cannot parse push remote URL: %w", err)
	}

	switch info.protocol {
	case protocolSSH:
		// SCP format: git@host:owner/repo.git
		return fmt.Sprintf("git@%s:%s.git", info.host, checkpointRepo), nil
	case protocolHTTPS:
		return fmt.Sprintf("https://%s/%s.git", info.host, checkpointRepo), nil
	default:
		return "", fmt.Errorf("unsupported protocol %q in push remote", info.protocol)
	}
}

// extractOwnerFromRemoteURL extracts the owner from a git remote URL.
// Returns empty string if the URL cannot be parsed.
func extractOwnerFromRemoteURL(rawURL string) string {
	info, err := parseGitRemoteURL(rawURL)
	if err != nil {
		return ""
	}
	return info.owner
}

// getRemoteURL returns the URL configured for a git remote.
func getRemoteURL(ctx context.Context, remoteName string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", remoteName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("remote %q not found", remoteName)
	}
	return strings.TrimSpace(string(output)), nil
}

// redactURL removes credentials from a URL for safe logging.
// Handles both HTTPS URLs with embedded credentials and general URLs.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// For non-URL formats (SSH SCP), just return the host portion
		if idx := strings.Index(rawURL, "@"); idx >= 0 {
			if colonIdx := strings.Index(rawURL[idx:], ":"); colonIdx >= 0 {
				return rawURL[idx+1:idx+colonIdx] + ":***"
			}
		}
		return "<unparseable>"
	}
	u.User = nil
	u.RawQuery = ""
	host := u.Host
	path := u.Path
	return u.Scheme + "://" + host + path
}
