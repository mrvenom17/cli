export interface Env {
	GITHUB_TOKEN: string;
}

const RUN_URL_PATTERN = /^https:\/\/github\.com\/entireio\/cli\/actions\/runs\/\d+$/;
const WORKFLOW_ID = "e2e-triage.yml";
const REPO = "entireio/cli";

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);
		if (url.pathname !== "/triage") {
			return new Response("Not found", { status: 404 });
		}

		const runURL = url.searchParams.get("run_url");
		const slackChannel = url.searchParams.get("slack_channel") ?? "";
		const slackThreadTS = url.searchParams.get("slack_thread_ts") ?? "";

		if (!runURL || !RUN_URL_PATTERN.test(runURL)) {
			return new Response("Invalid or missing run_url parameter", { status: 400 });
		}

		const resp = await fetch(
			`https://api.github.com/repos/${REPO}/actions/workflows/${WORKFLOW_ID}/dispatches`,
			{
				method: "POST",
				headers: {
					Authorization: `Bearer ${env.GITHUB_TOKEN}`,
					Accept: "application/vnd.github+json",
					"User-Agent": "e2e-triage-trigger-worker",
				},
				body: JSON.stringify({
					ref: "main",
					inputs: {
						run_url: runURL,
						slack_channel: slackChannel,
						slack_thread_ts: slackThreadTS,
					},
				}),
			},
		);

		if (!resp.ok) {
			const body = await resp.text();
			return new Response(`GitHub API error: ${resp.status} ${body}`, { status: 502 });
		}

		return new Response(
			`<!DOCTYPE html>
<html><body>
<h2>Triage started</h2>
<p>The E2E triage workflow has been dispatched. Check Slack for results.</p>
<p><a href="${runURL}">View original run</a></p>
</body></html>`,
			{ headers: { "Content-Type": "text/html; charset=utf-8" } },
		);
	},
} satisfies ExportedHandler<Env>;
