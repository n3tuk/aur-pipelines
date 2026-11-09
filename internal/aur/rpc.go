// Package aur provides a client for the Arch User Repository (AUR) RPC
// interface and helpers for resolving package metadata and dependencies.
//
// The client targets version 5 of the aurweb RPC interface, querying the
// `info` endpoint to retrieve package details (including the package base and
// dependency lists) and to determine whether a given name exists in the AUR.
package aur

type (
	// response is the envelope returned by every aurweb RPC v5 query. The
	// Results field carries the per-package details for info queries; for
	// error responses it is empty and Error holds the message.
	response struct {
		// Version is the RPC interface version the server responded with.
		Version int `json:"version"`
		// Type is the response type ("multiinfo" or "error").
		Type string `json:"type"`
		// ResultCount is the number of entries in Results.
		ResultCount int `json:"resultcount"`
		// Results holds the per-package details for a successful info query.
		Results []Package `json:"results"`
		// Error holds the error message when Type is "error".
		Error string `json:"error"`
	}

	// Package holds the details of a single AUR package as returned by the
	// info endpoint. Only the fields required by aur-pipelines are modelled;
	// the RPC interface returns additional fields which are ignored.
	//
	// Fields that a package does not have are omitted by the server, so slice
	// fields may be nil.
	Package struct {
		// Name is the package name (the exact name queried for).
		Name string `json:"Name"`
		// PackageBase is the name of the package base this package belongs to.
		// For a single (non-split) package this equals Name.
		PackageBase string `json:"PackageBase"`
		// Version is the package version string (including pkgrel and epoch).
		Version string `json:"Version"`
		// Depends is the list of runtime dependencies.
		Depends []string `json:"Depends"`
		// MakeDepends is the list of build-time dependencies.
		MakeDepends []string `json:"MakeDepends"`
		// CheckDepends is the list of test-time dependencies.
		CheckDepends []string `json:"CheckDepends"`
		// OptDepends is the list of optional dependencies. It is retained for
		// completeness but is not followed during dependency resolution.
		OptDepends []string `json:"OptDepends"`
		// Provides is the list of virtual packages or alternative names this
		// package provides.
		Provides []string `json:"Provides"`
		// Conflicts is the list of packages this package conflicts with.
		Conflicts []string `json:"Conflicts"`
		// Replaces is the list of packages this package replaces.
		Replaces []string `json:"Replaces"`
	}
)

// RPC interface version and response envelope type values.
const (
	// rpcVersion is the version of the aurweb RPC interface this client
	// targets.
	rpcVersion = 5
	// typeMultiInfo is the response type for a successful info query.
	typeMultiInfo = "multiinfo"
	// typeError is the response type for a failed query.
	typeError = "error"
)
