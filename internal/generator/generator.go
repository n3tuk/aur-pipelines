// Package generator assembles Concourse CI pipelines from a resolved set of
// AUR package bases. Each package base becomes its own pipeline containing a
// build-upload job, a standardised sign job, and a serial repository job, with
// all object-storage interactions modelled as s3 resources so that Concourse's
// version tracking drives job triggering and passed constraints.
//
// Secrets are never embedded: the GPG signing key and passphrase, the R2
// credentials, and the notification webhook URL are all referenced as
// Concourse credential-manager lookups (for example "((gpg-signing-key))") and
// resolved by Concourse at run time.
package generator

import (
	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/pipeline"
	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

type (
	// Generator produces Concourse pipelines from configuration and a resolved
	// dependency set. Construct one with New.
	Generator struct {
		config *config.Config
	}

	// Pipeline pairs a generated Concourse pipeline with the package base it
	// was generated for, so callers can name the output (for example the file
	// to write it to).
	Pipeline struct {
		// PackageBase is the package base this pipeline builds.
		PackageBase string
		// Pipeline is the generated Concourse pipeline.
		Pipeline pipeline.Pipeline
	}
)

const (
	// Concourse credential-manager references for the secrets the pipelines
	// need. These are flat names resolved by Concourse against its configured
	// team and pipeline credential paths; aur-pipelines never sees their
	// values.
	secretGPGKey        = "((gpg-signing-key))"        //nolint:gosec // credential-manager reference, not a secret
	secretGPGPassphrase = "((gpg-signing-passphrase))" //nolint:gosec // credential-manager reference, not a secret
	secretR2AccessKey   = "((r2-access-key-id))"       //nolint:gosec // credential-manager reference, not a secret
	secretR2SecretKey   = "((r2-secret-access-key))"   //nolint:gosec // credential-manager reference, not a secret
	secretWebhookURL    = "((webhook-url))"            //nolint:gosec // credential-manager reference, not a secret

	// Job names used within every generated package pipeline.
	jobBuildUpload = "build-upload"
	jobSign        = "sign"
	jobRepository  = "repository"

	// Resource names used within every generated package pipeline.
	resourceSource     = "source"
	resourceArtefacts  = "build-artefacts"
	resourceSignatures = "signatures"
	resourceRepository = "repository-db"

	// serialGroupRepository is the serial group applied to repository jobs so
	// that repository-database updates are serialised (within a pipeline; see
	// the generated documentation for the cross-pipeline caveat).
	serialGroupRepository = "repository"

	// platformLinux is the Concourse task platform for all generated tasks.
	platformLinux = "linux"
)

// New constructs a Generator using the given configuration.
func New(cfg *config.Config) *Generator {
	return &Generator{config: cfg}
}

// Generate produces one pipeline per package base in the resolved result,
// ordered to match the result's deterministic package-base ordering.
func (g *Generator) Generate(result *resolver.Result) []Pipeline {
	pipelines := make([]Pipeline, 0, len(result.PackageBases))

	// Index dependency edges by dependent base so each pipeline can wire the
	// cross-pipeline triggers for its own dependencies.
	dependenciesOf := make(map[string][]string)
	for _, edge := range result.Edges {
		dependenciesOf[edge.From] = append(dependenciesOf[edge.From], edge.To)
	}

	for _, base := range result.PackageBases {
		pipelines = append(pipelines, Pipeline{
			PackageBase: base,
			Pipeline:    g.pipelineFor(base, dependenciesOf[base]),
		})
	}

	return pipelines
}
