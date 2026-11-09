package generator

import (
	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/pipeline"
)

const (
	// buildScript builds the AUR package in a fresh, up-to-date environment
	// and stages the built package files into the build-artefacts output
	// directory.
	buildScript = `set -euo pipefail
pacman-key --init
pacman-key --populate archlinux
pacman -Syu --noconfirm
useradd -m builder
cp -r source /home/builder/source
chown -R builder:builder /home/builder/source
sudo -u builder bash -c 'cd /home/builder/source && makepkg --syncdeps --noconfirm --skippgpcheck'
cp /home/builder/source/*.pkg.tar.zst build-artefacts/`

	// signScript imports the signing key from the credential-manager-provided
	// environment variables and produces a detached, armoured signature for
	// every built package file, writing them into the signatures output
	// directory. "--armor" is the literal GPG flag name, not UK "armour".
	//
	//nolint:misspell // "--armor" is the literal GPG flag name
	signScript = `set -euo pipefail
echo "${GPG_PRIVATE_KEY}" | gpg --batch --import
for package in build-artefacts/*.pkg.tar.zst; do
  echo "${GPG_PASSPHRASE}" \
    | gpg --batch --yes --pinentry-mode loopback --passphrase-fd 0 \
        --detach-sign --armor \
        --output "signatures/$(basename "${package}").sig" "${package}"
done`

	// repositoryScript adds the built and signed packages to the repository
	// database and rewrites the bucket-friendly database copies. Because object
	// storage does not support symlinks, the .db and .files entries are written
	// as copies of their .tar.gz counterparts rather than symlinks.
	repositoryScript = `set -euo pipefail
cp build-artefacts/*.pkg.tar.zst repository-db/
cp signatures/*.sig repository-db/ 2>/dev/null || true
cd repository-db
repo-add "${REPOSITORY}.db.tar.gz" ./*.pkg.tar.zst
cp -f "${REPOSITORY}.db.tar.gz" "${REPOSITORY}.db"
cp -f "${REPOSITORY}.files.tar.gz" "${REPOSITORY}.files"`
)

// buildUploadJob builds the package and uploads the built artefacts. When the
// package has AUR dependencies, the job also gets the shared repository
// database with trigger enabled, so the package is rebuilt once a dependency
// has been published to the repository.
func (g *Generator) buildUploadJob(dependencies []string) pipeline.Job {
	plan := []pipeline.Step{
		{Get: resourceSource, Trigger: true},
	}

	if len(dependencies) > 0 {
		// Triggering on the repository database (updated by the serial
		// repository job) ensures a dependent package only rebuilds once its
		// dependency is installable from the repository, avoiding a race with
		// the raw package upload.
		plan = append(plan, pipeline.Step{Get: resourceRepository, Trigger: true})
	}

	plan = append(plan,
		pipeline.Step{
			Task: "build",
			Config: &pipeline.TaskConfig{
				Platform:      platformLinux,
				ImageResource: g.image(g.config.Container.Build),
				Inputs:        []pipeline.Input{{Name: resourceSource}},
				Outputs:       []pipeline.Output{{Name: resourceArtefacts}},
				Run:           shell(buildScript),
			},
		},
		pipeline.Step{Put: resourceArtefacts},
	)

	return pipeline.Job{
		Name: jobBuildUpload,
		Plan: plan,
	}
}

// signJob fetches freshly uploaded build artefacts, signs them with the
// credential-manager-provided key, and uploads the signature files. The job is
// standardised across all package pipelines. It is not serialised.
func (g *Generator) signJob() pipeline.Job {
	return pipeline.Job{
		Name: jobSign,
		Plan: []pipeline.Step{
			{Get: resourceArtefacts, Trigger: true, Passed: []string{jobBuildUpload}},
			{
				Task: "sign",
				Params: map[string]string{
					"GPG_PRIVATE_KEY": secretGPGKey,
					"GPG_PASSPHRASE":  secretGPGPassphrase,
				},
				Config: &pipeline.TaskConfig{
					Platform:      platformLinux,
					ImageResource: g.image(g.config.Container.Sign),
					Inputs:        []pipeline.Input{{Name: resourceArtefacts}},
					Outputs:       []pipeline.Output{{Name: resourceSignatures}},
					Run:           shell(signScript),
				},
			},
			{Put: resourceSignatures},
		},
	}
}

// repositoryJob adds the built and signed packages to the repository database
// and republishes it. It is the serial job: repository-database updates must be
// performed sequentially so concurrent runs cannot clobber each other.
func (g *Generator) repositoryJob() pipeline.Job {
	plan := []pipeline.Step{
		{Get: resourceArtefacts, Passed: []string{jobBuildUpload}},
		{Get: resourceSignatures, Trigger: true, Passed: []string{jobSign}},
		{Get: resourceRepository},
	}

	// The notification tasks read build metadata from the meta resource, so it
	// must be fetched in the job when notifications are configured.
	if g.hasWebhooks(config.WebhookTypeBuild) {
		plan = append(plan, pipeline.Step{Get: resourceMeta})
	}

	plan = append(plan,
		pipeline.Step{
			Task: "repo-add",
			Params: map[string]string{
				"REPOSITORY": g.config.Bucket.Repository,
			},
			Config: &pipeline.TaskConfig{
				Platform:      platformLinux,
				ImageResource: g.image(g.config.Container.Upload),
				Inputs: []pipeline.Input{
					{Name: resourceArtefacts},
					{Name: resourceSignatures},
					{Name: resourceRepository},
				},
				Outputs: []pipeline.Output{{Name: resourceRepository}},
				Run:     shell(repositoryScript),
			},
		},
		pipeline.Step{Put: resourceRepository},
	)

	return pipeline.Job{
		Name:         jobRepository,
		Serial:       true,
		SerialGroups: []string{serialGroupRepository},
		OnSuccess:    g.notifyStep(config.WebhookTypeBuild, config.WebhookWhenSuccess),
		OnFailure:    g.notifyStep(config.WebhookTypeBuild, config.WebhookWhenFailure),
		Plan:         plan,
	}
}

// image builds the task image_resource for the given configured container
// image, using registry-image so any OCI registry image can be used. When the
// image has a Secret configured, registry pull credentials are sourced from the
// corresponding "((repositories/<secret>.username))" and
// "((repositories/<secret>.password))" credentials.
func (g *Generator) image(img config.Image) *pipeline.ImageResource {
	source := map[string]string{sourceRepository: img.Image}
	if img.Tag != "" {
		source["tag"] = img.Tag
	}

	if img.Secret != "" {
		source["username"] = repositoryUsernameRef(img.Secret)
		source["password"] = repositoryPasswordRef(img.Secret)
	}

	return &pipeline.ImageResource{
		Type:   typeRegistryImage,
		Source: source,
	}
}

// repositoryUsernameRef and repositoryPasswordRef build the Concourse
// credential-manager references for a container registry's pull credentials
// from the image's secret name.
func repositoryUsernameRef(secret string) string {
	return "((repositories/" + secret + ".username))"
}

func repositoryPasswordRef(secret string) string {
	return "((repositories/" + secret + ".password))"
}

// shell wraps a script body in a bash -c invocation.
func shell(script string) pipeline.Command {
	return pipeline.Command{
		Path: "bash",
		Args: []string{"-c", script},
	}
}
