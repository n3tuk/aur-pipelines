package cli_test

import (
	"strings"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/cli"
)

func testBuildInfo() cli.BuildInfo {
	return cli.BuildInfo{
		Branch:       "main",
		Commit:       "abc1234",
		Version:      "1.2.3",
		BuildDate:    "2026-09-05T12:00:00Z",
		Architecture: "amd64",
	}
}

func TestBuildInfoString(t *testing.T) {
	t.Parallel()

	got := testBuildInfo().String()
	want := "aur-pipelines 1.2.3 (branch: main, commit: abc1234, built: 2026-09-05T12:00:00Z, arch: amd64)"

	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestBuildInfoReport(t *testing.T) {
	t.Parallel()

	var builder strings.Builder

	err := testBuildInfo().Report(&builder)
	if err != nil {
		t.Fatalf("Report() returned unexpected error: %v", err)
	}

	got := builder.String()

	cases := map[string]string{
		"Version":      "1.2.3",
		"Branch":       "main",
		"Commit":       "abc1234",
		"Build Date":   "2026-09-05T12:00:00Z",
		"Architecture": "amd64",
	}

	for label, value := range cases {
		line := label + ":"
		if !strings.Contains(got, line) {
			t.Errorf("Report() output missing label %q\noutput:\n%s", label, got)
		}

		if !strings.Contains(got, value) {
			t.Errorf("Report() output missing value %q for %q\noutput:\n%s", value, label, got)
		}
	}

	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Report() output should end with a newline, got %q", got)
	}
}

func TestBuildInfoReportOrdering(t *testing.T) {
	t.Parallel()

	var builder strings.Builder

	err := testBuildInfo().Report(&builder)
	if err != nil {
		t.Fatalf("Report() returned unexpected error: %v", err)
	}

	got := builder.String()

	order := []string{"Version", "Branch", "Commit", "Build Date", "Architecture"}
	last := -1

	for _, label := range order {
		idx := strings.Index(got, label+":")
		if idx == -1 {
			t.Fatalf("Report() output missing label %q", label)
		}

		if idx < last {
			t.Errorf("Report() label %q out of expected order\noutput:\n%s", label, got)
		}

		last = idx
	}
}
