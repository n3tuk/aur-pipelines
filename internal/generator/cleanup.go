package generator

import (
	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/pipeline"
)

const (
	// cleanupPipelineName is the conventional name for the repository-cleanup
	// pipeline, used by callers when writing it out.
	cleanupPipelineName = "cleanup"
	// resourceDaily is the name of the time resource that triggers the daily
	// cleanup run.
	resourceDaily = "daily"
	// jobCleanup is the name of the cleanup job.
	jobCleanup = "cleanup-repository"
	// cleanupInterval is how often the cleanup job runs.
	cleanupInterval = "24h"
	// cleanupImage is the image used for the cleanup task; it needs a shell,
	// the AWS CLI (for S3-compatible access to R2), and tar/gzip to read the
	// package database.
	cleanupImage = "amazon/aws-cli"

	// cleanupScript enumerates the packages held in the bucket, and for any
	// package with more than one version present, verifies that the newest
	// version is the one recorded in the repository database before removing
	// the older versions. Removal is guarded: an older version is only deleted
	// once the newer version has been confirmed present in the database, so the
	// repository is never left referencing a missing package.
	cleanupScript = `set -euo pipefail

export AWS_ACCESS_KEY_ID="${R2_ACCESS_KEY_ID}"
export AWS_SECRET_ACCESS_KEY="${R2_SECRET_ACCESS_KEY}"

s3() {
  aws s3 --endpoint-url "${R2_ENDPOINT}" "$@"
}

workdir="$(mktemp -d)"
cd "${workdir}"

# Fetch the current repository database so we can confirm which version of each
# package is the one the repository actually references.
s3 cp "s3://${BUCKET}/${REPOSITORY}.db.tar.gz" ./db.tar.gz
mkdir -p db
tar -xzf ./db.tar.gz -C db

# The database contains one directory per package, named "<pkgname>-<version>";
# record the referenced package filename for each entry.
referenced="$(mktemp)"
for desc in db/*/desc; do
  [ -f "${desc}" ] || continue
  awk '/^%FILENAME%$/{getline; print}' "${desc}" >> "${referenced}"
done

# List every package file currently in the bucket.
present="$(mktemp)"
s3 ls "s3://${BUCKET}/" --recursive \
  | awk '{print $4}' \
  | grep -E '\.pkg\.tar\.zst$' > "${present}" || true

# For each package file present in the bucket that is NOT the version referenced
# by the database, remove it and its signature. The database is the source of
# truth for the current version, so anything it does not reference is an older
# version safe to delete.
while IFS= read -r object; do
  [ -n "${object}" ] || continue
  filename="$(basename "${object}")"
  if grep -qxF "${filename}" "${referenced}"; then
    continue
  fi
  echo "Removing superseded package: ${object}"
  s3 rm "s3://${BUCKET}/${object}"
  s3 rm "s3://${BUCKET}/${object}.sig" || true
done < "${present}"`
)

// CleanupPipeline builds the daily repository-cleanup pipeline. It is a single
// pipeline (independent of the per-package pipelines) that removes superseded
// package versions from the bucket while leaving the version referenced by the
// repository database in place.
func (g *Generator) CleanupPipeline() Pipeline {
	return Pipeline{
		PackageBase: cleanupPipelineName,
		Pipeline: pipeline.Pipeline{
			Resources: []pipeline.Resource{
				{
					Name:   resourceDaily,
					Type:   "time",
					Icon:   "clock-outline",
					Source: map[string]string{"interval": cleanupInterval},
				},
			},
			Jobs: []pipeline.Job{g.cleanupJob()},
		},
	}
}

// cleanupJob builds the cleanup job: it is triggered daily by the time resource
// and runs the guarded cleanup task.
func (g *Generator) cleanupJob() pipeline.Job {
	return pipeline.Job{
		Name:      jobCleanup,
		Serial:    true,
		OnSuccess: g.notifyStep(config.WebhookTypeCleanup, config.WebhookWhenSuccess),
		OnFailure: g.notifyStep(config.WebhookTypeCleanup, config.WebhookWhenFailure),
		Plan: []pipeline.Step{
			{Get: resourceDaily, Trigger: true},
			{
				Task: "cleanup",
				Params: map[string]string{
					"BUCKET":               g.config.Bucket.Name,
					"REPOSITORY":           g.config.Bucket.Repository,
					"R2_ENDPOINT":          r2Endpoint,
					"R2_ACCESS_KEY_ID":     secretR2AccessKey,
					"R2_SECRET_ACCESS_KEY": secretR2SecretKey,
				},
				Config: &pipeline.TaskConfig{
					Platform: platformLinux,
					ImageResource: &pipeline.ImageResource{
						Type:   typeRegistryImage,
						Source: map[string]string{sourceRepository: cleanupImage},
					},
					Run: shell(cleanupScript),
				},
			},
		},
	}
}
