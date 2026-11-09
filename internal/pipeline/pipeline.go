// Package pipeline provides a typed model of a Concourse CI pipeline and
// marshals it to the YAML understood by `fly set-pipeline`.
//
// The model is intentionally programmatic (Go structs marshalled to YAML)
// rather than template-based, so generated pipelines are type-safe and cannot
// drift from a textual template. Secret references (for example
// "((gpg.signing-key))") and pass-through notification templates (for example
// "{{ .PackageName }}") are ordinary string values and are emitted verbatim;
// aur-pipelines does not interpret them.
package pipeline

type (
	// Pipeline is a complete Concourse pipeline definition.
	Pipeline struct {
		// ResourceTypes declares any custom resource types used by the
		// pipeline's resources.
		ResourceTypes []ResourceType `yaml:"resource_types,omitempty"`
		// Resources declares the resources the jobs get from and put to.
		Resources []Resource `yaml:"resources,omitempty"`
		// Jobs declares the pipeline's jobs.
		Jobs []Job `yaml:"jobs"`
	}

	// ResourceType declares a custom Concourse resource type.
	ResourceType struct {
		// Name is the resource type name referenced by resources.
		Name string `yaml:"name"`
		// Type is the underlying type used to fetch the resource type image
		// (for example "registry-image").
		Type string `yaml:"type"`
		// Source is the configuration used to locate the resource type image.
		Source map[string]string `yaml:"source,omitempty"`
	}

	// Resource declares a Concourse resource.
	Resource struct {
		// Name is the resource name referenced by get/put steps.
		Name string `yaml:"name"`
		// Type is the resource type (built-in or a declared resource type).
		Type string `yaml:"type"`
		// Icon is an optional Material Design icon name for the web UI.
		Icon string `yaml:"icon,omitempty"`
		// Source is the resource-type-specific configuration.
		Source map[string]string `yaml:"source,omitempty"`
	}

	// Job is a Concourse job: an ordered plan of steps, optionally serialised.
	Job struct {
		// Name is the job name.
		Name string `yaml:"name"`
		// Serial, when true, prevents concurrent runs of the job.
		Serial bool `yaml:"serial,omitempty"`
		// SerialGroups, when set, serialises the job against other jobs
		// sharing any of the named groups.
		SerialGroups []string `yaml:"serial_groups,omitempty"`
		// Plan is the ordered list of steps the job runs.
		Plan []Step `yaml:"plan"`
		// OnSuccess runs when the plan succeeds.
		OnSuccess *Step `yaml:"on_success,omitempty"`
		// OnFailure runs when the plan fails.
		OnFailure *Step `yaml:"on_failure,omitempty"`
		// OnError runs when the plan errors.
		OnError *Step `yaml:"on_error,omitempty"`
		// OnAbort runs when the plan is aborted.
		OnAbort *Step `yaml:"on_abort,omitempty"`
		// Ensure runs after the plan regardless of outcome.
		Ensure *Step `yaml:"ensure,omitempty"`
	}

	// Step is a single step in a job plan. Exactly one of Get, Put, or Task is
	// set to select the step type; the remaining fields are modifiers that
	// apply to the selected step type.
	Step struct {
		// Get names a resource to fetch (a get step).
		Get string `yaml:"get,omitempty"`
		// Put names a resource to update (a put step).
		Put string `yaml:"put,omitempty"`
		// Task names an inline task (a task step); Config supplies its body.
		Task string `yaml:"task,omitempty"`

		// Resource overrides the resource a get/put step targets when it
		// differs from the step name.
		Resource string `yaml:"resource,omitempty"`
		// Passed constrains a get step to versions that passed the named jobs.
		Passed []string `yaml:"passed,omitempty"`
		// Trigger, on a get step, starts a new build when a new version
		// appears.
		Trigger bool `yaml:"trigger,omitempty"`
		// Params supplies resource- or task-specific parameters.
		Params map[string]string `yaml:"params,omitempty"`
		// Config is the inline task configuration for a task step.
		Config *TaskConfig `yaml:"config,omitempty"`

		// OnSuccess runs when the step succeeds.
		OnSuccess *Step `yaml:"on_success,omitempty"`
		// OnFailure runs when the step fails.
		OnFailure *Step `yaml:"on_failure,omitempty"`
		// Ensure runs after the step regardless of outcome.
		Ensure *Step `yaml:"ensure,omitempty"`
	}

	// TaskConfig is the inline configuration of a task step.
	TaskConfig struct {
		// Platform is the platform the task runs on (typically "linux").
		Platform string `yaml:"platform"`
		// ImageResource describes the container image the task runs in.
		ImageResource *ImageResource `yaml:"image_resource,omitempty"`
		// Inputs are the named inputs the task requires.
		Inputs []Input `yaml:"inputs,omitempty"`
		// Outputs are the named outputs the task produces.
		Outputs []Output `yaml:"outputs,omitempty"`
		// Params are environment variables provided to the task.
		Params map[string]string `yaml:"params,omitempty"`
		// Run is the command the task executes.
		Run Command `yaml:"run"`
	}

	// ImageResource describes the image a task runs in.
	ImageResource struct {
		// Type is the resource type used to fetch the image (for example
		// "registry-image").
		Type string `yaml:"type"`
		// Source is the configuration used to locate the image.
		Source map[string]string `yaml:"source,omitempty"`
	}

	// Input is a named task input.
	Input struct {
		// Name is the input name.
		Name string `yaml:"name"`
	}

	// Output is a named task output.
	Output struct {
		// Name is the output name.
		Name string `yaml:"name"`
	}

	// Command is the executable a task runs.
	Command struct {
		// Path is the path to the executable.
		Path string `yaml:"path"`
		// Args are the arguments passed to the executable.
		Args []string `yaml:"args,omitempty"`
		// Dir is the working directory the command runs in.
		Dir string `yaml:"dir,omitempty"`
	}
)
