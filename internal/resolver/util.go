package resolver

import (
	"cmp"
	"slices"
)

// dedupeStrings returns the unique, non-empty strings from the input,
// preserving the order of first appearance.
func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			continue
		}

		_, ok := seen[value]
		if ok {
			continue
		}

		seen[value] = struct{}{}
		unique = append(unique, value)
	}

	return unique
}

// sortedKeys returns the keys of a set (map to bool) as a sorted slice. Keys
// mapped to false are excluded.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))

	for key, present := range set {
		if present {
			keys = append(keys, key)
		}
	}

	slices.Sort(keys)

	return keys
}

// compareStrings orders two strings, for use with slices.SortFunc.
func compareStrings(a, b string) int {
	return cmp.Compare(a, b)
}
