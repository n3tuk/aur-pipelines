package aur

import "context"

// ByName indexes a slice of packages by their Name for convenient lookup. When
// the server returns duplicate names (which it should not for an info query),
// the last entry wins.
func ByName(packages []Package) map[string]Package {
	index := make(map[string]Package, len(packages))
	for _, pkg := range packages {
		index[pkg.Name] = pkg
	}

	return index
}

// Members queries the AUR for the given names and returns the subset that
// exist in the AUR, preserving the input order and de-duplicating. This is the
// membership check used to decide whether a dependency should be treated as an
// AUR package (and therefore resolved and built) or ignored as an official
// repository package.
func (c *Client) Members(ctx context.Context, names ...string) ([]string, error) {
	packages, err := c.Info(ctx, names...)
	if err != nil {
		return nil, err
	}

	index := ByName(packages)

	members := make([]string, 0, len(packages))
	seen := make(map[string]struct{}, len(packages))

	for _, name := range names {
		_, isMember := index[name]
		if !isMember {
			continue
		}

		_, alreadyAdded := seen[name]
		if alreadyAdded {
			continue
		}

		seen[name] = struct{}{}
		members = append(members, name)
	}

	return members, nil
}
