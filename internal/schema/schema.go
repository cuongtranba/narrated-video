// Package schema holds the published contract for video.config.yaml.
//
// It is its own package so the validator does not depend on the project
// template: the schema is what the tool enforces, the template is only one
// document that happens to satisfy it.
package schema

import _ "embed"

const FileName = "video.schema.json"

//go:embed video.schema.json
var File []byte
