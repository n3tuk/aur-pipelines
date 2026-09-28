package aur

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type (
	// SrcInfo is the parsed representation of an AUR package's .SRCINFO file.
	// It captures the package base, its version, the dependency and provides
	// information aggregated across the base and all split packages, and the
	// names of the individual packages produced by the build.
	SrcInfo struct {
		// PackageBase is the pkgbase value: the name of the package base.
		PackageBase string
		// Version is the composed version string (epoch:pkgver-pkgrel), using
		// only the parts that are present.
		Version string
		// Packages is the list of pkgname values produced by this base.
		Packages []string
		// Depends is the union of runtime dependencies (constraints stripped).
		Depends []string
		// MakeDepends is the union of build-time dependencies.
		MakeDepends []string
		// CheckDepends is the union of test-time dependencies.
		CheckDepends []string
		// Provides is the union of provided names (constraints stripped).
		Provides []string
	}

	// srcInfoAccumulator collects raw field values while parsing, before they
	// are composed into a SrcInfo.
	srcInfoAccumulator struct {
		pkgbase      string
		epoch        string
		pkgver       string
		pkgrel       string
		packages     []string
		depends      []string
		makeDepends  []string
		checkDepends []string
		provides     []string
	}
)

// ParseSrcInfo parses a .SRCINFO document from the reader into a SrcInfo. It
// aggregates the dependency and provides fields across the pkgbase section and
// every pkgname section, strips version constraints from dependency and
// provides entries, and handles architecture-suffixed field names (for
// example "depends_x86_64"). Unknown fields, blank lines, and comments are
// ignored. It returns an error only if reading the input fails.
func ParseSrcInfo(reader io.Reader) (*SrcInfo, error) {
	acc := &srcInfoAccumulator{}
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		key, value, ok := splitField(scanner.Text())
		if !ok {
			continue
		}

		acc.add(key, value)
	}

	err := scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("reading .SRCINFO: %w", err)
	}

	return acc.build(), nil
}

// splitField splits a .SRCINFO line into its key and value. It returns
// ok=false for blank lines, comments, and lines without a "key = value"
// structure.
func splitField(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}

	name, val, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}

	return strings.TrimSpace(name), strings.TrimSpace(val), true
}

// baseKey strips any architecture suffix (for example "_x86_64") from a field
// key so that, for example, "depends_x86_64" is treated the same as "depends".
func baseKey(key string) string {
	name, _, found := strings.Cut(key, "_")
	if found {
		return name
	}

	return key
}

// stripConstraint removes any version constraint or comparison from a
// dependency or provides entry, returning just the package name. For example
// "foo>=1.2" and "bar=1.0" both become their bare name, and "baz: a description"
// (an optdepends style) is reduced to "baz".
func stripConstraint(entry string) string {
	// optdepends use "name: description"; keep only the name.
	name, _, found := strings.Cut(entry, ":")
	if found {
		entry = name
	}

	// Strip the first version comparison operator and everything after it.
	idx := strings.IndexAny(entry, "<>=")
	if idx >= 0 {
		entry = entry[:idx]
	}

	return strings.TrimSpace(entry)
}

// add records a single field into the accumulator.
func (a *srcInfoAccumulator) add(key, value string) {
	if value == "" {
		return
	}

	switch baseKey(key) {
	case "pkgbase":
		a.pkgbase = value
	case "pkgname":
		a.packages = append(a.packages, value)
	case "epoch":
		a.epoch = value
	case "pkgver":
		a.pkgver = value
	case "pkgrel":
		a.pkgrel = value
	case "depends":
		a.depends = append(a.depends, stripConstraint(value))
	case "makedepends":
		a.makeDepends = append(a.makeDepends, stripConstraint(value))
	case "checkdepends":
		a.checkDepends = append(a.checkDepends, stripConstraint(value))
	case "provides":
		a.provides = append(a.provides, stripConstraint(value))
	}
}

// build composes the accumulated fields into a SrcInfo, de-duplicating list
// fields and composing the version string.
func (a *srcInfoAccumulator) build() *SrcInfo {
	return &SrcInfo{
		PackageBase:  a.pkgbase,
		Version:      composeVersion(a.epoch, a.pkgver, a.pkgrel),
		Packages:     dedupe(a.packages),
		Depends:      dedupe(a.depends),
		MakeDepends:  dedupe(a.makeDepends),
		CheckDepends: dedupe(a.checkDepends),
		Provides:     dedupe(a.provides),
	}
}

// composeVersion builds the version string from its parts, in the form
// "epoch:pkgver-pkgrel", omitting the epoch and pkgrel when absent.
func composeVersion(epoch, pkgver, pkgrel string) string {
	if pkgver == "" {
		return ""
	}

	version := pkgver

	if epoch != "" {
		version = epoch + ":" + version
	}

	if pkgrel != "" {
		version += "-" + pkgrel
	}

	return version
}
