package pipeline_test

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/n3tuk/aur-pipelines/internal/pipeline"
)

const keyRepository = "repository"

func TestMarshalMinimalPipeline(t *testing.T) {
	t.Parallel()

	p := &pipeline.Pipeline{
		Jobs: []pipeline.Job{
			{
				Name: "build",
				Plan: []pipeline.Step{
					{Get: "kalc-bin", Trigger: true},
				},
			},
		},
	}

	got := marshal(t, p)

	want := `---
jobs:
  - name: build
    plan:
      - get: kalc-bin
        trigger: true
`

	if got != want {
		t.Errorf("Marshal() mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestMarshalResourceWithSecretSource(t *testing.T) {
	t.Parallel()

	p := &pipeline.Pipeline{
		Resources: []pipeline.Resource{
			{
				Name: keyRepository,
				Type: "s3",
				Icon: "cloud-upload",
				//nolint:gosec // values are Concourse credential-manager references, not hardcoded secrets
				Source: map[string]string{
					"access_key_id":     "((r2.access-key-id))",
					"secret_access_key": "((r2.secret-access-key))",
					"bucket":            "your-bucket-name",
				},
			},
		},
		Jobs: []pipeline.Job{
			{Name: "noop", Plan: []pipeline.Step{{Get: keyRepository}}},
		},
	}

	got := marshal(t, p)

	// Secret references must be emitted verbatim and unquoted; map keys are
	// deterministically ordered (alphabetical).
	want := `---
resources:
  - name: repository
    type: s3
    icon: cloud-upload
    source:
      access_key_id: ((r2.access-key-id))
      bucket: your-bucket-name
      secret_access_key: ((r2.secret-access-key))
jobs:
  - name: noop
    plan:
      - get: repository
`

	if got != want {
		t.Errorf("Marshal() mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestMarshalSerialSignJobWithPassed(t *testing.T) {
	t.Parallel()

	p := &pipeline.Pipeline{
		Jobs: []pipeline.Job{
			{
				Name:   "upload",
				Serial: true,
				Plan: []pipeline.Step{
					{Get: "signed", Trigger: true, Passed: []string{"sign"}},
					{
						Task: "upload-packages",
						Params: map[string]string{
							"ACCESS_KEY": "((r2.access-key-id))",
						},
						Config: &pipeline.TaskConfig{
							Platform: "linux",
							ImageResource: &pipeline.ImageResource{
								Type:   "registry-image",
								Source: map[string]string{keyRepository: "archlinux", "tag": "base-devel"},
							},
							Inputs: []pipeline.Input{{Name: "signed"}},
							Run:    pipeline.Command{Path: "bash", Args: []string{"-c", "repo-add"}},
						},
					},
				},
			},
		},
	}

	got := marshal(t, p)

	want := `---
jobs:
  - name: upload
    serial: true
    plan:
      - get: signed
        passed:
          - sign
        trigger: true
      - task: upload-packages
        params:
          ACCESS_KEY: ((r2.access-key-id))
        config:
          platform: linux
          image_resource:
            type: registry-image
            source:
              repository: archlinux
              tag: base-devel
          inputs:
            - name: signed
          run:
            path: bash
            args:
              - -c
              - repo-add
`

	if got != want {
		t.Errorf("Marshal() mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestMarshalPreservesTemplateVerbatim(t *testing.T) {
	t.Parallel()

	tmpl := `{{ if eq .PipelineStatus "succeeded" }} ok {{ else }} fail {{ end }}`

	p := &pipeline.Pipeline{
		Jobs: []pipeline.Job{
			{
				Name: "notify",
				Plan: []pipeline.Step{
					{
						Task:   "send",
						Params: map[string]string{"TEMPLATE": tmpl},
						Config: &pipeline.TaskConfig{Platform: "linux", Run: pipeline.Command{Path: "curl"}},
					},
				},
			},
		},
	}

	got := marshal(t, p)

	// The template content must survive round-tripping exactly. YAML may quote
	// it, but the inner text must be unchanged.
	if !strings.Contains(got, `.PipelineStatus`) {
		t.Errorf("template not preserved in output:\n%s", got)
	}

	restored := roundTrip(t, got)

	params := restored.Jobs[0].Plan[0].Params
	if params["TEMPLATE"] != tmpl {
		t.Errorf("template value after round-trip = %q, want %q", params["TEMPLATE"], tmpl)
	}
}

func TestMarshalResourceTypes(t *testing.T) {
	t.Parallel()

	p := &pipeline.Pipeline{
		ResourceTypes: []pipeline.ResourceType{
			{
				Name:   "ntfy",
				Type:   "registry-image",
				Source: map[string]string{keyRepository: "example/ntfy-resource"},
			},
		},
		Jobs: []pipeline.Job{{Name: "noop", Plan: []pipeline.Step{{Get: "x"}}}},
	}

	got := marshal(t, p)

	want := `---
resource_types:
  - name: ntfy
    type: registry-image
    source:
      repository: example/ntfy-resource
jobs:
  - name: noop
    plan:
      - get: x
`

	if got != want {
		t.Errorf("Marshal() mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func marshal(t *testing.T, p *pipeline.Pipeline) string {
	t.Helper()

	out, err := p.Marshal()
	if err != nil {
		t.Fatalf("Marshal() returned unexpected error: %v", err)
	}

	return string(out)
}

func roundTrip(t *testing.T, data string) pipeline.Pipeline {
	t.Helper()

	var restored pipeline.Pipeline

	err := yaml.Unmarshal([]byte(data), &restored)
	if err != nil {
		t.Fatalf("unmarshalling generated YAML: %v", err)
	}

	return restored
}
