package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/pipeline"
)

// notifyImage is the minimal image used for the notification task; it only
// needs a shell and curl.
const notifyImage = "curlimages/curl"

// notifyStep builds a notification task step for the given pipeline status
// ("succeeded" or "failed"). It emits one curl invocation per configured
// webhook, passing the configured headers and body template through verbatim
// for Concourse to interpolate at run time. It returns nil when no webhooks are
// configured.
func (g *Generator) notifyStep(status string) *pipeline.Step {
	if len(g.config.Webhooks) == 0 {
		return nil
	}

	params := map[string]string{
		"WEBHOOK_URL":     secretWebhookURL,
		"PIPELINE_STATUS": status,
	}

	return &pipeline.Step{
		Task:   "notify-" + status,
		Params: params,
		Config: &pipeline.TaskConfig{
			Platform: platformLinux,
			ImageResource: &pipeline.ImageResource{
				Type:   typeRegistryImage,
				Source: map[string]string{sourceRepository: notifyImage},
			},
			// curlimages/curl is Alpine-based and provides sh, not bash.
			Run: pipeline.Command{Path: "sh", Args: []string{"-c", g.notifyScript()}},
		},
	}
}

// notifyScript renders the shell script that sends each configured webhook. The
// header values and body template are pass-through strings and are emitted
// verbatim; Concourse interpolates any templating at run time.
func (g *Generator) notifyScript() string {
	var builder strings.Builder

	builder.WriteString("set -euo pipefail\n")

	for _, webhook := range g.config.Webhooks {
		builder.WriteString(curlCommand(webhook))
		builder.WriteString("\n")
	}

	return strings.TrimRight(builder.String(), "\n")
}

// curlCommand renders a single curl invocation for a webhook, including its
// headers and body template. Each argument is placed on its own line using
// shell line-continuations, so the generated script reads as a readable
// multi-line block rather than one long line. The webhook URL is taken from the
// credential reference in the environment rather than the (potentially secret)
// configured URL, so it is not embedded in the pipeline.
func curlCommand(webhook config.Webhook) string {
	lines := []string{"curl --fail --silent --show-error"}

	headers := append([]config.Header(nil), webhook.Headers...)
	sort.SliceStable(headers, func(i, j int) bool {
		return headers[i].Name < headers[j].Name
	})

	for _, header := range headers {
		lines = append(lines, "--header "+quote(fmt.Sprintf("%s: %s", header.Name, header.Value)))
	}

	if webhook.Template != "" {
		lines = append(lines, "--data "+quote(webhook.Template))
	}

	lines = append(lines, `"${WEBHOOK_URL}"`)

	// Join with a trailing backslash and newline so the command spans multiple
	// lines; continuation lines are indented by two spaces for readability.
	return strings.Join(lines, " \\\n  ")
}

// quote wraps a value in single quotes for safe inclusion in a shell command,
// escaping any embedded single quotes. Pass-through templating is preserved.
func quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
