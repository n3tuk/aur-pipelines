package generator

import (
	"fmt"

	"github.com/n3tuk/aur-pipelines/internal/pipeline"
)

const (
	// r2Endpoint is the S3-compatible endpoint template for Cloudflare R2. The
	// account is supplied via a credential-manager reference so it is not
	// embedded.
	r2Endpoint = "https://((r2.account-id)).r2.cloudflarestorage.com"
	// r2Region is the region string R2 expects for S3-compatible access.
	r2Region = "auto"
)

// pipelineFor builds the complete Concourse pipeline for a single package base,
// wiring cross-pipeline triggers for each of the given dependency bases.
func (g *Generator) pipelineFor(base string, dependencies []string) pipeline.Pipeline {
	return pipeline.Pipeline{
		Resources: g.resourcesFor(base),
		Jobs: []pipeline.Job{
			g.buildUploadJob(dependencies),
			g.signJob(),
			g.repositoryJob(),
		},
	}
}

// resourcesFor builds the resources used by a package base's pipeline: the AUR
// source repository, the build-artefacts and signatures object-storage
// resources, and the shared repository database.
func (g *Generator) resourcesFor(base string) []pipeline.Resource {
	return []pipeline.Resource{
		{
			Name: resourceSource,
			Type: "git",
			Icon: "git",
			Source: map[string]string{
				"uri": fmt.Sprintf("https://aur.archlinux.org/%s.git", base),
			},
		},
		g.s3Resource(resourceArtefacts, "package-upload", fmt.Sprintf("%s/%s-.*\\.pkg\\.tar\\.zst", base, base)),
		g.s3Resource(resourceSignatures, "cloud-lock", fmt.Sprintf("%s/%s-.*\\.pkg\\.tar\\.zst\\.sig", base, base)),
		g.s3Resource(resourceRepository, "database", g.config.Bucket.Repository+"\\.db\\.tar\\.gz"),
	}
}

// s3Resource builds an s3-typed resource for the given name and versioned-file
// regexp, using the configured bucket and the R2 credential references.
func (g *Generator) s3Resource(name, icon, regexp string) pipeline.Resource {
	return pipeline.Resource{
		Name: name,
		Type: "s3",
		Icon: icon,
		Source: map[string]string{
			"bucket":            g.config.Bucket.Name,
			"region_name":       r2Region,
			"endpoint":          r2Endpoint,
			"access_key_id":     secretR2AccessKey,
			"secret_access_key": secretR2SecretKey,
			"regexp":            regexp,
		},
	}
}
