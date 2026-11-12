// Package schema holds the JSON Schemas for permidex's cache files. The schemas are embedded so each
// refresh can write them beside the data they describe.
package schema

import _ "embed"

// GCPRoles is the schema for the GCP cache, schema/gcp-roles.schema.json.
//
//go:embed gcp-roles.schema.json
var GCPRoles []byte
