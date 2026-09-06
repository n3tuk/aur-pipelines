package config

// Default container images used when a stage (or the whole container block) is
// omitted from the configuration.
const (
	// defaultBuildImage/defaultBuildTag build and sign packages using the Arch
	// Linux base-devel image, which provides pacman, makepkg, and gpg.
	defaultBuildImage = "archlinux"
	defaultBuildTag   = "base-devel"

	// defaultCleanupImage/defaultCleanupTag run the cleanup job; the AWS CLI
	// image provides S3-compatible access to R2.
	defaultCleanupImage = "amazon/aws-cli"
	defaultCleanupTag   = "latest"

	// defaultNotifyImage runs the notification tasks; it provides a POSIX shell
	// and curl. It is intentionally untagged, matching upstream's rolling tag.
	defaultNotifyImage = "curlimages/curl"
	defaultNotifyTag   = ""
)

// applyDefaults fills any container image that was left unset with its default,
// so the rest of the application always sees a fully populated configuration.
func (c *Config) applyDefaults() {
	c.Container.Build = c.Container.Build.orDefault(defaultBuildImage, defaultBuildTag)
	c.Container.Sign = c.Container.Sign.orDefault(defaultBuildImage, defaultBuildTag)
	c.Container.Upload = c.Container.Upload.orDefault(defaultBuildImage, defaultBuildTag)
	c.Container.Cleanup = c.Container.Cleanup.orDefault(defaultCleanupImage, defaultCleanupTag)
	c.Container.Notify = c.Container.Notify.orDefault(defaultNotifyImage, defaultNotifyTag)
}

// orDefault returns the image with its Image and Tag filled from the given
// defaults when they are empty, preserving any configured Secret.
func (i Image) orDefault(image, tag string) Image {
	if i.Image == "" {
		i.Image = image
		// Only default the tag when the image itself was defaulted, so a
		// configured image without a tag is not given an unrelated default tag.
		if i.Tag == "" {
			i.Tag = tag
		}
	}

	return i
}
