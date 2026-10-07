package generator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	// pipelineFileExt is the extension used for generated pipeline files.
	pipelineFileExt = ".yaml"
	// outputDirPerm is the permission mode for the output directory.
	outputDirPerm = 0o755
	// outputFilePerm is the permission mode for generated pipeline files.
	outputFilePerm = 0o644
)

// FileName returns the file name a pipeline is written to, derived from its
// package base (for example "kalc-bin.yaml").
func (p Pipeline) FileName() string {
	return p.PackageBase + pipelineFileExt
}

// WriteDir marshals every pipeline and writes it to the given directory, one
// file per pipeline named after its package base. The directory is created if
// it does not exist. Existing files are overwritten, so repeated runs are
// idempotent. It returns the sorted list of file names written.
func WriteDir(dir string, pipelines []Pipeline) ([]string, error) {
	err := os.MkdirAll(dir, outputDirPerm)
	if err != nil {
		return nil, fmt.Errorf("creating output directory %q: %w", dir, err)
	}

	written := make([]string, 0, len(pipelines))

	for _, p := range pipelines {
		data, marshalErr := p.Pipeline.Marshal()
		if marshalErr != nil {
			return nil, fmt.Errorf("marshalling pipeline %q: %w", p.PackageBase, marshalErr)
		}

		path := filepath.Join(dir, p.FileName())

		writeErr := os.WriteFile(path, data, outputFilePerm)
		if writeErr != nil {
			return nil, fmt.Errorf("writing pipeline %q: %w", path, writeErr)
		}

		written = append(written, p.FileName())
	}

	sort.Strings(written)

	return written, nil
}

// WriteStream marshals every pipeline and writes them to the given writer,
// separated by a comment naming each pipeline's file. It is used for dry runs,
// where no files are written to disk.
func WriteStream(out io.Writer, pipelines []Pipeline) error {
	for _, p := range pipelines {
		data, err := p.Pipeline.Marshal()
		if err != nil {
			return fmt.Errorf("marshalling pipeline %q: %w", p.PackageBase, err)
		}

		_, err = fmt.Fprintf(out, "# %s\n%s\n", p.FileName(), data)
		if err != nil {
			return fmt.Errorf("writing pipeline %q: %w", p.PackageBase, err)
		}
	}

	return nil
}
