// Package config defines the structure of the aur-pipelines input
// configuration file and provides loading (and, in a later task, JSON-schema
// validation) of that configuration from a YAML file on disk.
//
// The configuration mirrors the structure documented in the project README:
// a set of container images to use for each pipeline stage, the target bucket
// repository, one or more notification webhooks, and the list of AUR packages
// to build pipelines for.
package config

type (
	// Config is the top-level aur-pipelines configuration, loaded from a YAML
	// file. Each field corresponds to a top-level key in that file.
	Config struct {
		// Container holds the container images used for each pipeline stage. It
		// is optional; any omitted stage (or the whole block) falls back to a
		// built-in default image.
		Container Container `json:"container,omitzero" mapstructure:"container" yaml:"container"`
		// Bucket describes the target object-storage bucket and repository.
		Bucket Bucket `json:"bucket" jsonschema:"required" mapstructure:"bucket" yaml:"bucket"`
		// Webhooks is the list of notification webhooks to invoke on completion.
		Webhooks []Webhook `json:"webhook,omitempty" mapstructure:"webhook" yaml:"webhook"`
		// Packages is the list of AUR packages to generate pipelines for.
		Packages []Package `json:"packages" jsonschema:"required,minItems=1" mapstructure:"packages" yaml:"packages"`
	}

	// Container groups the container image references used by the distinct
	// stages of a generated pipeline. Every field is optional; an omitted stage
	// falls back to a built-in default image (see DefaultContainer).
	Container struct {
		// Build is the image used for the package build stage.
		Build Image `json:"build,omitzero" mapstructure:"build" yaml:"build"`
		// Sign is the image used for the package signing stage.
		Sign Image `json:"sign,omitzero" mapstructure:"sign" yaml:"sign"`
		// Upload is the image used for the repository upload stage.
		Upload Image `json:"upload,omitzero" mapstructure:"upload" yaml:"upload"`
		// Cleanup is the image used for the daily repository-cleanup job. It
		// must provide the AWS CLI (for S3-compatible access to R2) in addition
		// to a shell and the usual archive tools.
		Cleanup Image `json:"cleanup,omitzero" mapstructure:"cleanup" yaml:"cleanup"`
		// Notify is the image used for the notification tasks. It must provide
		// a POSIX shell and curl.
		Notify Image `json:"notify,omitzero" mapstructure:"notify" yaml:"notify"`
	}

	// Image is a container image reference expressed as a repository image name
	// and a tag, with an optional credential for pulling it from a private
	// registry. The image and tag are optional; when omitted, the stage's
	// default image is used.
	Image struct {
		// Image is the container image repository name (e.g. "archlinux").
		Image string `json:"image,omitempty" mapstructure:"image" yaml:"image"`
		// Tag is the container image tag (e.g. "base-devel").
		Tag string `json:"tag,omitempty" mapstructure:"tag" yaml:"tag"`
		// Secret optionally names the credential holding the registry username
		// and password used to pull the image. When set, the pull credentials
		// are sourced from "((repositories/<secret>.username))" and
		// "((repositories/<secret>.password))"; when unset, the image is pulled
		// anonymously.
		//
		//nolint:lll // struct tags cannot be wrapped
		Secret string `json:"secret,omitempty" jsonschema:"pattern=^[a-zA-Z0-9._-]+$" mapstructure:"secret" yaml:"secret,omitempty"`
	}

	// Bucket describes the target object-storage bucket holding the Arch
	// package repository, together with the repository (database) name within
	// it.
	Bucket struct {
		// Name is the name of the object-storage bucket.
		Name string `json:"name" jsonschema:"required,minLength=1" mapstructure:"name" yaml:"name"`
		// Repository is the Arch repository (database) name within the bucket.
		Repository string `json:"repository" jsonschema:"required,minLength=1" mapstructure:"repository" yaml:"repository"`
	}

	// Webhook describes a single notification endpoint invoked when a pipeline
	// completes. It is selected for a job by its Type (which kind of pipeline)
	// and When (which job outcome). The header values and template are
	// pass-through strings in which shell-style variable references (for
	// example "${PACKAGE_NAME}") are expanded at run time; aur-pipelines does
	// not otherwise interpret them.
	Webhook struct {
		// Name is a human-readable identifier for the webhook (e.g. "ntfy").
		Name string `json:"name" jsonschema:"required,minLength=1" mapstructure:"name" yaml:"name"`
		// Type selects which kind of pipeline this webhook applies to: "build"
		// for the per-package pipelines, or "cleanup" for the daily
		// repository-cleanup pipeline.
		Type string `json:"type" jsonschema:"required,enum=build,enum=cleanup" mapstructure:"type" yaml:"type"`
		// When selects which job outcome this webhook fires on: "on_success" or
		// "on_failure".
		When string `json:"when" jsonschema:"required,enum=on_success,enum=on_failure" mapstructure:"when" yaml:"when"`
		// Secret is the name of the credential holding this webhook's details.
		// It is used to build a Concourse credential-manager reference of the
		// form "((webhooks/<secret>.url))", so each webhook has its own secret
		// (with room for additional fields, such as authentication, in future)
		// and no endpoint is exposed in the pipeline configuration.
		Secret string `json:"secret" jsonschema:"required,pattern=^[a-zA-Z0-9._-]+$" mapstructure:"secret" yaml:"secret"`
		// Headers is the ordered list of HTTP headers to send with the request.
		Headers []Header `json:"headers,omitempty" mapstructure:"headers" yaml:"headers"`
		// Template is the pass-through notification body template.
		Template string `json:"template,omitempty" mapstructure:"template" yaml:"template"`
	}

	// Header is a single HTTP header name/value pair sent with a webhook
	// request. The value is a pass-through string in which shell-style variable
	// references are expanded at run time.
	Header struct {
		// Name is the HTTP header field name.
		Name string `json:"name" jsonschema:"required,minLength=1" mapstructure:"name" yaml:"name"`
		// Value is the HTTP header field value (may contain variables).
		Value string `json:"value" jsonschema:"required" mapstructure:"value" yaml:"value"`
	}

	// Package is a single AUR package entry to generate a pipeline for. The
	// shape is intentionally minimal for now, leaving room for future
	// per-package overrides or options.
	Package struct {
		// Name is the AUR package name.
		Name string `json:"name" jsonschema:"required,minLength=1" mapstructure:"name" yaml:"name"`
	}
)

// Webhook Type and When values. Type selects which kind of pipeline a webhook
// applies to; When selects which job outcome it fires on.
const (
	// WebhookTypeBuild selects the per-package build pipelines.
	WebhookTypeBuild = "build"
	// WebhookTypeCleanup selects the daily repository-cleanup pipeline.
	WebhookTypeCleanup = "cleanup"

	// WebhookWhenSuccess fires the webhook when the job succeeds.
	WebhookWhenSuccess = "on_success"
	// WebhookWhenFailure fires the webhook when the job fails.
	WebhookWhenFailure = "on_failure"
)
