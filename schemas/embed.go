// Package schemas embeds the JSON Schemas that validate AIBox's declarative files
// (profiles, model catalog, benchmark results). The .json files are the source of truth;
// this package only makes them available to the CLI.
package schemas

import "embed"

// FS holds every schema file in this directory.
//
//go:embed *.json
var FS embed.FS

// Schema file names.
const (
	Profile = "profile.schema.json"
	Catalog = "catalog.schema.json"
)
