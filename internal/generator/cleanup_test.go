package generator_test

import (
	"strings"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/generator"
)

func TestCleanupPipelineGolden(t *testing.T) {
	t.Parallel()

	p := generator.New(testConfig()).CleanupPipeline()

	out, err := p.Pipeline.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	assertGolden(t, "cleanup.yaml", string(out))
}

func TestCleanupPipelineName(t *testing.T) {
	t.Parallel()

	p := generator.New(testConfig()).CleanupPipeline()
	if p.PackageBase != "cleanup" {
		t.Errorf("cleanup pipeline name = %q, want %q", p.PackageBase, "cleanup")
	}
}

func TestCleanupIsDailyTriggered(t *testing.T) {
	t.Parallel()

	out := cleanupYAML(t)

	if !strings.Contains(out, "type: time") {
		t.Errorf("cleanup pipeline should use a time resource:\n%s", out)
	}

	if !strings.Contains(out, "interval: 24h") {
		t.Errorf("cleanup pipeline should trigger every 24h:\n%s", out)
	}

	// The cleanup job must be triggered by the time resource.
	if !strings.Contains(out, "get: daily\n        trigger: true") {
		t.Errorf("cleanup job should be triggered by the daily time resource:\n%s", out)
	}
}

func TestCleanupIsSerial(t *testing.T) {
	t.Parallel()

	out := cleanupYAML(t)
	if !strings.Contains(out, "serial: true") {
		t.Errorf("cleanup job should be serial:\n%s", out)
	}
}

func TestCleanupUsesR2Credentials(t *testing.T) {
	t.Parallel()

	out := cleanupYAML(t)

	for _, secret := range []string{"((r2.access-key-id))", "((r2.secret-access-key))"} {
		if !strings.Contains(out, secret) {
			t.Errorf("cleanup pipeline should reference %q:\n%s", secret, out)
		}
	}
}

func TestCleanupRemovalIsGuardedByDatabase(t *testing.T) {
	t.Parallel()

	out := cleanupYAML(t)

	// The script must download and read the repository database before any
	// removal, and only remove files not referenced by the database.
	dbFetch := strings.Index(out, ".db.tar.gz")
	removal := strings.Index(out, "s3 rm")

	if dbFetch == -1 || removal == -1 {
		t.Fatalf("cleanup script missing database fetch or removal:\n%s", out)
	}

	if dbFetch > removal {
		t.Errorf("database must be read before any removal:\n%s", out)
	}

	if !strings.Contains(out, "grep -qxF") {
		t.Errorf("removal must be guarded by a database-reference check:\n%s", out)
	}
}

func cleanupYAML(t *testing.T) string {
	t.Helper()

	p := generator.New(testConfig()).CleanupPipeline()

	out, err := p.Pipeline.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	return string(out)
}
