# Slack-Triggered E2E Triage & Fix Pipeline

When E2E tests fail on `main`, a Slack alert is posted with a clickable "Run Triage" link. Clicking it triggers the triage workflow via a Cloudflare Worker, which triages failures, generates a fix plan, and offers a "Fix It" link that auto-applies fixes and opens a draft PR.

## Flow

```
E2E fails → Slack alert → "Run Triage" → e2e-triage.yml
  → triage (claude-code-action, read-only) → plan generation (claude-code-action, read-only)
  → plan in GH summary + Slack "Fix It" link
  → user clicks "Fix It" → Worker /fix → e2e-fix.yml
  → claude-code-action (write tools) → applies fixes + creates draft PR → Slack
```

1. `.github/workflows/e2e.yml` posts a failure alert to Slack using the bot token (via `chat.postMessage`), then posts a threaded "Run Triage" link that encodes the run URL and Slack thread context.
2. A user clicks the link, which hits the Cloudflare Worker at `e2e-triage.entireio.workers.dev/triage`.
3. The Worker validates the `run_url` and calls `workflow_dispatch` on `.github/workflows/e2e-triage.yml` via the GitHub API.
4. The triage workflow:
   - Downloads E2E artifacts via `scripts/download-e2e-artifacts.sh`
   - Runs the `/e2e:triage-ci` skill via `anthropics/claude-code-action@v1` (read-only tools)
   - Extracts triage output from the execution file and writes it to `triage.md`
   - Runs the `/e2e:implement` skill via `claude-code-action` (read-only tools) to generate a fix plan
   - Posts the plan to GitHub step summary (via `display_report: true`) and uploads it as an artifact
   - Posts a "Fix It" link to the Slack thread
5. A user clicks the "Fix It" link, which hits the Worker at `e2e-triage.entireio.workers.dev/fix`.
6. The Worker dispatches `.github/workflows/e2e-fix.yml` with the triage run ID and agent list.
7. The fix workflow:
   - Downloads plan artifacts from the triage run
   - Runs `claude-code-action` with write tools to apply fixes, run verification (`fmt`, `lint`, `test:e2e:canary`), and create a draft PR
   - Posts the PR link (or failure details) to the Slack thread

## Cloudflare Worker

Source lives in the infra repo at `cloudflare/workers/e2e-triage-trigger/`.

### `/triage` endpoint

Accepts a GET request with query parameters:

- `run_url` (required) — must match `https://github.com/entireio/cli/actions/runs/\d+`
- `slack_channel` — Slack channel ID for thread replies
- `slack_thread_ts` — Slack thread timestamp for thread replies

Dispatches `e2e-triage.yml` with these values as `workflow_dispatch` inputs.

### `/fix` endpoint

Accepts a GET request with query parameters:

- `triage_run_id` (required) — numeric run ID of the triage workflow
- `run_url` (required) — original failed E2E run URL
- `failed_agents` (required) — comma-separated list of agents to fix
- `slack_channel` — Slack channel ID for thread replies
- `slack_thread_ts` — Slack thread timestamp for thread replies

Dispatches `e2e-fix.yml` with these values as `workflow_dispatch` inputs.

**Secret:** `GITHUB_TOKEN` — a PAT with `actions:write` scope, stored in Cloudflare secrets (`wrangler secret put GITHUB_TOKEN`).

## Workflows

### `e2e-triage.yml`

Triages E2E failures and generates fix plans. Uses a matrix strategy (one job per failed agent).

**Claude invocations** (both via `anthropics/claude-code-action@v1`):

| Step | Skill | Tools | Output |
|------|-------|-------|--------|
| Run triage | `/e2e:triage-ci` | Read, Grep, Glob | triage.md (artifact + GH summary) |
| Generate fix plan | `/e2e:implement` | Read, Grep, Glob | plan.md (artifact + GH summary) |

The `/e2e:implement` skill enters plan mode first (read-only tools prevent actual file changes), producing a detailed fix plan without applying anything.

### `e2e-fix.yml`

Applies fix plans and creates a draft PR. Single job (not matrix) since fixes may touch shared test infrastructure.

**Claude invocation** (via `anthropics/claude-code-action@v1`):

| Step | Tools | Output |
|------|-------|--------|
| Apply fixes | Edit, Write, Read, Glob, Grep, Bash(git:\*), Bash(mise:\*), Bash(gh:\*) | Branch + draft PR |

Claude reads the plan artifacts, applies fixes, runs `mise run fmt && mise run lint && mise run test:e2e:canary`, then creates a `fix/e2e-<run_id>` branch and opens a draft PR.

## Slack Setup

The Slack app needs:

- `chat:write` scope — to post alerts and triage results
- Bot must be invited to the alert channel

No event subscriptions or incoming webhooks are needed.

## GitHub Config

**Repository variables:**

- `E2E_SLACK_CHANNEL` — Slack channel ID where failure alerts are posted

**Repository secrets:**

- `SLACK_BOT_TOKEN` — Slack bot token with `chat:write` scope
- `ANTHROPIC_API_KEY` — for Claude triage and fix steps

The built-in `${{ github.token }}` is used for GitHub API calls within workflows.

## Manual Fallback

### Triage

Run `.github/workflows/e2e-triage.yml` manually with `workflow_dispatch`:

- `run_url` (required) — the failed run URL
- `sha` — commit SHA (auto-detected from run if omitted)
- `failed_agents` — comma-separated list (auto-detected from run if omitted)
- `slack_channel` — for Slack thread replies
- `slack_thread_ts` — for Slack thread replies

### Fix

Run `.github/workflows/e2e-fix.yml` manually with `workflow_dispatch`:

- `triage_run_id` (required) — run ID of the triage workflow
- `run_url` (required) — original failed E2E run URL
- `failed_agents` (required) — comma-separated list of agents to fix
- `slack_channel` — for Slack thread replies
- `slack_thread_ts` — for Slack thread replies
