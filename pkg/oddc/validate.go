// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import "strings"

// Validation is the machine-readable result of validating a catalog.
type Validation struct {
	APIVersion    string   `json:"apiVersion"`
	SchemaVersion string   `json:"schemaVersion"`
	Revision      string   `json:"revision"`
	Valid         bool     `json:"valid"`
	Entities      int      `json:"entities"`
	Evidence      int      `json:"evidence"`
	Errors        []string `json:"errors"`
}

// Validate checks the catalog below root. It never fails itself; problems
// are reported in the result.
func Validate(root string) Validation {
	source := DirSource{Root: root}
	result := Validation{
		APIVersion:    EntityAPIVersion,
		SchemaVersion: SchemaVersion,
		Revision:      source.Revision(),
		Errors:        []string{},
	}

	registry, err := LoadRegistry(root)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	files, err := source.List("evidence")
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	for _, file := range files {
		if strings.HasSuffix(file, ".json") {
			result.Evidence++
		}
	}

	result.Errors = append(result.Errors, registry.Duplication()...)
	result.Errors = append(result.Errors, registry.securityErrors()...)
	result.Errors = append(result.Errors, registry.supportErrors()...)
	result.Valid = len(result.Errors) == 0
	result.Entities = len(registry.Entities)

	return result
}
