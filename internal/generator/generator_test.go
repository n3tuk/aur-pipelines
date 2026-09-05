package generator_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/generator"
	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

const (
	baseKalcBin = "kalc-bin"
	baseApp     = "app"
	baseLibDep  = "libdep"
	imageArch   = "archlinux"
	tagDevel    = "base-devel"
	webhookURL  = "https://ntfy.sh/example"
	headerTitle = "Title"
)

// update, when set via `go test -update`, rewrites the golden files instead of
// comparing against them.
//
//nolint:gochecknoglobals // test flags must be package-level variables
var update = flag.Bool("update", false, "update golden files")

func testConfig() *config.Config {
	return &config.Config{
		Container: config.Container{
			Build:  config.Image{Image: imageArch, Tag: tagDevel},
			Sign:   config.Image{Image: imageArch, Tag: tagDevel},
			Upload: config.Image{Image: imageArch, Tag: tagDevel},
		},
		Bucket: config.Bucket{Name: "my-bucket", Repository: "private"},
		Webhooks: []config.Webhook{
			{
				Name: "ntfy-success",
				Type: config.WebhookTypeBuild,
				When: config.WebhookWhenSuccess,
				URL:  webhookURL,
				Headers: []config.Header{
					{Name: headerTitle, Value: "${PACKAGE_NAME} built"},
					{Name: "Priority", Value: "low"},
				},
				Template: `Package ${PACKAGE_NAME} v${PACKAGE_VERSION} built. See ${PIPELINE_URL}.`,
			},
			{
				Name: "ntfy-failure",
				Type: config.WebhookTypeBuild,
				When: config.WebhookWhenFailure,
				URL:  webhookURL,
				Headers: []config.Header{
					{Name: headerTitle, Value: "${PACKAGE_NAME} failed"},
				},
				Template: `Package ${PACKAGE_NAME} failed. See ${PIPELINE_URL}.`,
			},
			{
				Name: "ntfy-cleanup",
				Type: config.WebhookTypeCleanup,
				When: config.WebhookWhenFailure,
				URL:  webhookURL,
				Headers: []config.Header{
					{Name: headerTitle, Value: "Cleanup of ${REPOSITORY} failed"},
				},
				Template: `Cleanup failed. See ${PIPELINE_URL}.`,
			},
		},
	}
}

func generatePipeline(t *testing.T, cfg *config.Config, result *resolver.Result, base string) string {
	t.Helper()

	pipelines := generator.New(cfg).Generate(result)

	for _, p := range pipelines {
		if p.PackageBase != base {
			continue
		}

		out, err := p.Pipeline.Marshal()
		if err != nil {
			t.Fatalf("Marshal() error: %v", err)
		}

		return string(out)
	}

	t.Fatalf("no pipeline generated for base %q", base)

	return ""
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name)

	if *update {
		err := os.WriteFile(path, []byte(got), 0o600)
		if err != nil {
			t.Fatalf("updating golden %q: %v", name, err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %q (run `go test -update`?): %v", name, err)
	}

	if got != string(want) {
		t.Errorf("generated pipeline for %q does not match golden; run `go test -update`\n--- got ---\n%s", name, got)
	}
}

func TestGenerateSinglePackageGolden(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{PackageBases: []string{baseKalcBin}}

	got := generatePipeline(t, testConfig(), result, baseKalcBin)

	assertGolden(t, "single.yaml", got)
}

func TestGeneratePackageWithDependencyGolden(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{
		PackageBases: []string{baseLibDep, baseApp},
		Edges:        []resolver.Edge{{From: baseApp, To: baseLibDep}},
	}

	got := generatePipeline(t, testConfig(), result, baseApp)

	assertGolden(t, "with-dependency.yaml", got)
}

func TestGenerateProducesOnePipelinePerBase(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{
		PackageBases: []string{baseLibDep, baseApp},
		Edges:        []resolver.Edge{{From: baseApp, To: baseLibDep}},
	}

	pipelines := generator.New(testConfig()).Generate(result)

	if len(pipelines) != 2 {
		t.Fatalf("generated %d pipelines, want 2", len(pipelines))
	}

	// Ordering must match the resolver's deterministic base ordering.
	if pipelines[0].PackageBase != baseLibDep || pipelines[1].PackageBase != baseApp {
		t.Errorf("pipeline order = [%s %s], want [libdep app]",
			pipelines[0].PackageBase, pipelines[1].PackageBase)
	}
}

func TestDependencyTriggerOnlyForDependents(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{
		PackageBases: []string{baseLibDep, baseApp},
		Edges:        []resolver.Edge{{From: baseApp, To: baseLibDep}},
	}

	// The dependent (app) build-upload job triggers on the repository db; the
	// dependency-free package (libdep) does not.
	app := generatePipeline(t, testConfig(), result, baseApp)

	appBuildStart := strings.Index(app, "name: build-upload")
	appSignStart := strings.Index(app, "name: sign\n")

	if appBuildStart == -1 || appSignStart == -1 || appSignStart < appBuildStart {
		t.Fatalf("could not locate build-upload/sign jobs:\n%s", app)
	}

	appBuild := app[appBuildStart:appSignStart]
	if !strings.Contains(appBuild, "get: repository-db") {
		t.Errorf("dependent pipeline should trigger build on repository-db:\n%s", appBuild)
	}

	lib := generatePipeline(t, testConfig(), result, baseLibDep)

	// libdep's build-upload plan should only get source (then build/put); it
	// must not trigger a build on the repository db.
	buildStart := strings.Index(lib, "name: build-upload")
	signStart := strings.Index(lib, "name: sign\n")

	if buildStart == -1 || signStart == -1 || signStart < buildStart {
		t.Fatalf("could not locate build-upload/sign jobs in pipeline:\n%s", lib)
	}

	buildPlan := lib[buildStart:signStart]
	if strings.Contains(buildPlan, "get: repository-db") {
		t.Errorf("dependency-free pipeline should not trigger build on repository-db:\n%s", buildPlan)
	}
}

func TestRepositoryJobIsSerial(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{PackageBases: []string{baseKalcBin}}
	got := generatePipeline(t, testConfig(), result, baseKalcBin)

	repoStart := strings.Index(got, "name: repository\n")
	if repoStart == -1 {
		t.Fatalf("repository job not found in pipeline:\n%s", got)
	}

	repoJob := got[repoStart:]
	if !strings.Contains(repoJob, "serial: true") {
		t.Errorf("repository job must be serial:\n%s", repoJob)
	}
}

func TestNoWebhooksOmitsNotification(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Webhooks = nil

	result := &resolver.Result{PackageBases: []string{baseKalcBin}}
	got := generatePipeline(t, cfg, result, baseKalcBin)

	if strings.Contains(got, "on_success") || strings.Contains(got, "notify") {
		t.Errorf("no webhooks configured, but notification present:\n%s", got)
	}
}

func TestSecretsAreCredentialReferences(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{PackageBases: []string{baseKalcBin}}
	got := generatePipeline(t, testConfig(), result, baseKalcBin)

	for _, secret := range []string{
		"((gpg.signing-key))",
		"((gpg.signing-passphrase))",
		"((r2.access-key-id))",
		"((r2.secret-access-key))",
		"((webhook.url))",
	} {
		if !strings.Contains(got, secret) {
			t.Errorf("expected credential reference %q in pipeline", secret)
		}
	}
}

func TestTemplatePassedThroughVerbatim(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{PackageBases: []string{baseKalcBin}}
	got := generatePipeline(t, testConfig(), result, baseKalcBin)

	// The webhook template variables must be present unrendered so the notify
	// task's shell expands them at run time.
	for _, variable := range []string{"${PACKAGE_NAME}", "${PACKAGE_VERSION}", "${PIPELINE_URL}"} {
		if !strings.Contains(got, variable) {
			t.Errorf("webhook template variable %q not passed through:\n%s", variable, got)
		}
	}

	// The task must export the variables it references.
	if !strings.Contains(got, `export PACKAGE_NAME=`) {
		t.Errorf("notify task should export PACKAGE_NAME:\n%s", got)
	}
}

func TestBuildAndFailureNotificationsAreSeparate(t *testing.T) {
	t.Parallel()

	result := &resolver.Result{PackageBases: []string{baseKalcBin}}
	got := generatePipeline(t, testConfig(), result, baseKalcBin)

	// Distinct success and failure notify tasks are generated, so no
	// conditional templating is needed.
	if !strings.Contains(got, "task: notify-succeeded") {
		t.Errorf("expected an on_success notify task:\n%s", got)
	}

	if !strings.Contains(got, "task: notify-failed") {
		t.Errorf("expected an on_failure notify task:\n%s", got)
	}
}
