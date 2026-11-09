package config

import (
	"fmt"
	"io"
	"strings"
)

// Summary renders a concise, human-readable overview of the loaded
// configuration and writes it to the provided writer. It is used by the
// generate command to confirm what was parsed before pipeline generation is
// implemented in later tasks. It returns any error encountered while writing.
func (c *Config) Summary(w io.Writer) error {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Bucket:     %s (repository: %s)\n", c.Bucket.Name, c.Bucket.Repository)
	fmt.Fprintf(&builder, "Build:      %s\n", c.Container.Build.Reference())
	fmt.Fprintf(&builder, "Sign:       %s\n", c.Container.Sign.Reference())
	fmt.Fprintf(&builder, "Upload:     %s\n", c.Container.Upload.Reference())

	fmt.Fprintf(&builder, "Webhooks:   %d\n", len(c.Webhooks))

	for _, webhook := range c.Webhooks {
		fmt.Fprintf(&builder, "  - %s (%d header(s))\n", webhook.Name, len(webhook.Headers))
	}

	fmt.Fprintf(&builder, "Packages:   %d\n", len(c.Packages))

	for _, pkg := range c.Packages {
		fmt.Fprintf(&builder, "  - %s\n", pkg.Name)
	}

	_, err := io.WriteString(w, builder.String())
	if err != nil {
		return fmt.Errorf("writing configuration summary: %w", err)
	}

	return nil
}

// Reference renders the image as a "image:tag" reference. When no tag is set,
// only the image name is returned.
func (i Image) Reference() string {
	if i.Tag == "" {
		return i.Image
	}

	return i.Image + ":" + i.Tag
}
