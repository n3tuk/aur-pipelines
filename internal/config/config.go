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
		// Container holds the container images used for each pipeline stage.
		Container Container `mapstructure:"container" yaml:"container"`
		// Bucket describes the target object-storage bucket and repository.
		Bucket Bucket `mapstructure:"bucket" yaml:"bucket"`
		// Webhooks is the list of notification webhooks to invoke on completion.
		Webhooks []Webhook `mapstructure:"webhook" yaml:"webhook"`
		// Packages is the list of AUR packages to generate pipelines for.
		Packages []Package `mapstructure:"packages" yaml:"packages"`
	}

	// Container groups the container image references used by the distinct
	// stages of a generated pipeline.
	Container struct {
		// Build is the image used for the package build stage.
		Build Image `mapstructure:"build" yaml:"build"`
		// Sign is the image used for the package signing stage.
		Sign Image `mapstructure:"sign" yaml:"sign"`
		// Upload is the image used for the repository upload stage.
		Upload Image `mapstructure:"upload" yaml:"upload"`
	}

	// Image is a container image reference expressed as a repository image
	// name and a tag.
	Image struct {
		// Image is the container image repository name (e.g. "archlinux").
		Image string `mapstructure:"image" yaml:"image"`
		// Tag is the container image tag (e.g. "base-devel").
		Tag string `mapstructure:"tag" yaml:"tag"`
	}

	// Bucket describes the target object-storage bucket holding the Arch
	// package repository, together with the repository (database) name within
	// it.
	Bucket struct {
		// Name is the name of the object-storage bucket.
		Name string `mapstructure:"name" yaml:"name"`
		// Repository is the Arch repository (database) name within the bucket.
		Repository string `mapstructure:"repository" yaml:"repository"`
	}

	// Webhook describes a single notification endpoint invoked when a pipeline
	// completes. The header values and template are opaque, pass-through
	// strings that may contain Concourse-rendered templating; aur-pipelines
	// does not interpret them.
	Webhook struct {
		// Name is a human-readable identifier for the webhook (e.g. "ntfy").
		Name string `mapstructure:"name" yaml:"name"`
		// URL is the endpoint the notification is sent to.
		URL string `mapstructure:"url" yaml:"url"`
		// Headers is the ordered list of HTTP headers to send with the request.
		Headers []Header `mapstructure:"headers" yaml:"headers"`
		// Template is the pass-through notification body template.
		Template string `mapstructure:"template" yaml:"template"`
	}

	// Header is a single HTTP header name/value pair sent with a webhook
	// request. The value is an opaque, pass-through string.
	Header struct {
		// Name is the HTTP header field name.
		Name string `mapstructure:"name" yaml:"name"`
		// Value is the HTTP header field value (may contain templating).
		Value string `mapstructure:"value" yaml:"value"`
	}

	// Package is a single AUR package entry to generate a pipeline for. The
	// shape is intentionally minimal for now, leaving room for future
	// per-package overrides or options.
	Package struct {
		// Name is the AUR package name.
		Name string `mapstructure:"name" yaml:"name"`
	}
)
