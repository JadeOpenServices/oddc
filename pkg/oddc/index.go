// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"path/filepath"
	"sort"
	"strings"
)

// Index lists every entity of a catalog for consumers that do not read
// the catalog tree themselves. It is generated, never committed.
type Index struct {
	APIVersion    string                `json:"apiVersion"`
	SchemaVersion string                `json:"schemaVersion"`
	Revision      string                `json:"revision"`
	Entities      map[string]IndexEntry `json:"entities"`
}

// IndexEntry describes one entity. Paths are slash-separated and relative
// to the catalog root.
type IndexEntry struct {
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	References []string `json:"references"`
	Evidence   []string `json:"evidence,omitempty"`
}

// Index describes the loaded catalog as of revision.
func (r *Registry) Index(revision string) (Index, error) {
	index := Index{
		APIVersion:    EntityAPIVersion,
		SchemaVersion: SchemaVersion,
		Revision:      revision,
		Entities:      make(map[string]IndexEntry, len(r.Entities)),
	}

	source := DirSource{Root: r.Root}

	for id, entity := range r.Entities {
		path, err := filepath.Rel(r.Root, r.paths[id])
		if err != nil {
			return Index{}, err
		}

		entry := IndexEntry{
			Kind:       entity.Kind,
			Name:       entity.Metadata.Name,
			Path:       filepath.ToSlash(path),
			References: collectRefs(entity.Data),
		}

		if entity.Kind == "DeviceModel" {
			files, err := source.List("evidence/" + id)
			if err != nil {
				return Index{}, err
			}
			for _, file := range files {
				if strings.HasSuffix(file, ".json") {
					entry.Evidence = append(entry.Evidence, file)
				}
			}
			sort.Strings(entry.Evidence)
		}

		index.Entities[id] = entry
	}

	return index, nil
}
