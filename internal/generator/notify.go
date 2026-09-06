package generator

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/pipeline"
)

const (
	// pipelineURLExpr builds the Concourse build URL from the standard build
	// metadata environment variables Concourse provides to every task.
	pipelineURLExpr = "${ATC_EXTERNAL_URL}/teams/${BUILD_TEAM_NAME}/pipelines/" +
		"${BUILD_PIPELINE_NAME}/jobs/${BUILD_JOB_NAME}/builds/${BUILD_NAME}"
)

// notifyStep builds a notification task step for the given webhook type
// (config.WebhookTypeBuild or config.WebhookTypeCleanup) and outcome
// (config.WebhookWhenSuccess or config.WebhookWhenFailure). It selects the
// configured webhooks matching that type and outcome and emits one curl
// invocation per webhook. It returns nil when no matching webhook is
// configured.
//
// Each webhook's endpoint is sourced from its own credential
// ("((webhooks/<secret>.url))") and exposed to the script as WEBHOOK_URL (or,
// when several webhooks share the same type and outcome, WEBHOOK_URL_1,
// WEBHOOK_URL_2, and so on). The notification variables (${PACKAGE_NAME},
// ${PACKAGE_VERSION}, ${PIPELINE_STATUS}, ${PIPELINE_URL}, and — for cleanup —
// ${REPOSITORY}) are computed by the task at run time, so the pass-through
// header values and body templates are expanded by the shell.
func (g *Generator) notifyStep(webhookType, when string) *pipeline.Step {
	matching := g.matchingWebhooks(webhookType, when)
	if len(matching) == 0 {
		return nil
	}

	status := statusFor(when)

	params := map[string]string{}
	for index, webhook := range matching {
		params[urlEnv(index, len(matching))] = webhookURLRef(webhook.Secret)
	}

	return &pipeline.Step{
		Task:   "notify-" + status,
		Params: params,
		Config: &pipeline.TaskConfig{
			Platform:      platformLinux,
			ImageResource: g.image(g.config.Container.Notify),
			// The notify image (curlimages/curl by default) is Alpine-based and
			// provides sh, not bash.
			Run: pipeline.Command{Path: "sh", Args: []string{"-c", g.notifyScriptFor(webhookType, status, matching)}},
		},
	}
}

// notifyScriptFor renders the notification script, supplying the generator's
// configured repository name for cleanup notifications.
func (g *Generator) notifyScriptFor(webhookType, status string, webhooks []config.Webhook) string {
	return notifyScript(webhookType, status, g.config.Bucket.Repository, webhooks)
}

// matchingWebhooks returns the configured webhooks matching the given type and
// outcome, in configuration order.
func (g *Generator) matchingWebhooks(webhookType, when string) []config.Webhook {
	matching := make([]config.Webhook, 0)

	for _, webhook := range g.config.Webhooks {
		if webhook.Type == webhookType && webhook.When == when {
			matching = append(matching, webhook)
		}
	}

	return matching
}

// statusFor maps a webhook "when" value to the human-readable status word used
// in the task name and the ${PIPELINE_STATUS} variable.
func statusFor(when string) string {
	if when == config.WebhookWhenFailure {
		return "failed"
	}

	return "succeeded"
}

// webhookURLRef builds the Concourse credential-manager reference for a
// webhook's URL from its secret name, of the form "((webhooks/<secret>.url))".
func webhookURLRef(secret string) string {
	return "((webhooks/" + secret + ".url))"
}

// urlEnv returns the environment-variable name carrying a webhook's URL. When
// only one webhook matches, the name is unadorned; when several match, each is
// suffixed with its (1-based) index to keep the names unique.
func urlEnv(index, count int) string {
	if count <= 1 {
		return "WEBHOOK_URL"
	}

	return "WEBHOOK_URL_" + strconv.Itoa(index+1)
}

// notifyScript renders the notification shell script for the given webhook type
// and status. It first exports the notification variables, then emits one curl
// invocation per matching webhook.
//
// Notifications are best-effort: the script does not abort on the first
// failure (no "set -e"/"pipefail"), and each curl invocation is guarded so that
// a webhook that is unreachable neither prevents the remaining webhooks from
// being sent nor fails the notification task. "set -u" is retained to catch
// genuinely unset variables.
func notifyScript(webhookType, status, repository string, webhooks []config.Webhook) string {
	var builder strings.Builder

	builder.WriteString("set -u\n")
	builder.WriteString(exportBlock(webhookType, status, repository))

	for index, webhook := range webhooks {
		builder.WriteString(curlCommand(webhook, index, len(webhooks)))
		builder.WriteString("\n")
	}

	return strings.TrimRight(builder.String(), "\n")
}

// exportBlock renders the shell statements that compute and export the
// notification variables available to the header values and body template. The
// build pipelines expose the package name and version; the cleanup pipeline
// exposes the repository name instead.
func exportBlock(webhookType, status, repository string) string {
	lines := []string{
		`export PIPELINE_STATUS="` + status + `"`,
		`export PIPELINE_URL="` + pipelineURLExpr + `"`,
	}

	switch webhookType {
	case config.WebhookTypeBuild:
		// The package name is the pipeline name; the version is derived from
		// the built package filename staged in the build-artefacts input.
		lines = append(lines,
			`export PACKAGE_NAME="${BUILD_PIPELINE_NAME}"`,
			`export PACKAGE_VERSION="$(ls build-artefacts/*.pkg.tar.zst 2>/dev/null `+
				`| head -n1 | sed -E 's|.*/[^-]+-([^-]+-[^-]+)-[^-]+\.pkg\.tar\.zst|\1|')"`,
		)
	case config.WebhookTypeCleanup:
		lines = append(lines, `export REPOSITORY="`+repository+`"`)
	}

	return strings.Join(lines, "\n") + "\n"
}

// curlCommand renders a single curl invocation for a webhook. Each argument is
// placed on its own line using shell line-continuations, so the generated
// script reads as a readable multi-line block. Header values and the body
// template are placed inside double quotes so the shell expands the exported
// notification variables; embedded double quotes are escaped. The invocation is
// guarded with "|| true" so a failed notification does not abort the others.
func curlCommand(webhook config.Webhook, index, count int) string {
	lines := []string{"curl --fail --silent --show-error"}

	headers := append([]config.Header(nil), webhook.Headers...)
	sort.SliceStable(headers, func(i, j int) bool {
		return headers[i].Name < headers[j].Name
	})

	for _, header := range headers {
		lines = append(lines, "--header "+dquote(fmt.Sprintf("%s: %s", header.Name, header.Value)))
	}

	if webhook.Template != "" {
		lines = append(lines, "--data "+dquote(webhook.Template))
	}

	lines = append(lines, `"${`+urlEnv(index, count)+`}" || true`)

	// Join with a trailing backslash and newline so the command spans multiple
	// lines; continuation lines are indented by two spaces for readability.
	return strings.Join(lines, " \\\n  ")
}

// dquote wraps a value in double quotes so the shell expands any variable
// references within it, escaping the characters that would otherwise break the
// quoting: backslashes, double quotes, and backticks.
func dquote(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"`", "\\`",
	)

	return `"` + replacer.Replace(value) + `"`
}
