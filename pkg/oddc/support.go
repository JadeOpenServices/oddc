// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

// Unsupported is a component of a model that cannot be used on Linux: its
// entity says so under support.linux, with the reason.
type Unsupported struct {
	Path   string
	ID     string
	Name   string
	Reason string
}

// UnsupportedComponents lists the components of a resolved model whose
// support.linux.status is "unsupported", by path.
func UnsupportedComponents(resolved map[string]any) []Unsupported {
	var found []Unsupported

	for _, component := range modelComponents(resolved) {
		object := componentAt(resolved, component.Path)
		if status, _ := Lookup(object, "support.linux.status"); status != "unsupported" {
			continue
		}

		name, _ := object["name"].(string)
		reason, _ := Lookup(object, "support.linux.reason")
		text, _ := reason.(string)
		found = append(found, Unsupported{Path: component.Path, ID: component.ID, Name: name, Reason: text})
	}

	return found
}
