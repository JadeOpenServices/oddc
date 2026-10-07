// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"fmt"
	"sort"
)

const SchemaVersion = "2.0.0"

type Provenance struct {
	Source string `json:"source"`
	Value  any    `json:"value"`
}

type Resolved struct {
	ModelID    string                  `json:"modelId"`
	Resolved   map[string]any          `json:"resolved"`
	Provenance map[string]Provenance   `json:"provenance"`
	History    map[string][]Provenance `json:"history"`
}

func (r *Registry) ResolveModel(
	modelID string,
	project []Overlay,
	host []Overlay,
) (Resolved, error) {
	entity, exists := r.Entities[modelID]
	if !exists {
		return Resolved{}, fmt.Errorf(
			"unknown model %q",
			modelID,
		)
	}

	if entity.Kind != "DeviceModel" {
		return Resolved{}, fmt.Errorf(
			"%q is kind %q, not DeviceModel",
			modelID,
			entity.Kind,
		)
	}

	expanded, err := r.ResolveEntity(modelID)
	if err != nil {
		return Resolved{}, err
	}

	value := cloneObject(expanded.Resolved)
	value["model"] = map[string]any{
		"id":   expanded.ID,
		"name": expanded.Name,
		"kind": expanded.Kind,
	}

	result := Resolved{
		ModelID:    modelID,
		Resolved:   value,
		Provenance: map[string]Provenance{},
		History:    map[string][]Provenance{},
	}

	owners := map[string]string{}

	if err := r.traceOwnership(
		entity.Data,
		"",
		modelID,
		map[string]bool{modelID: true},
		owners,
	); err != nil {
		return Resolved{}, err
	}

	owners["model.id"] = modelID
	owners["model.name"] = modelID
	owners["model.kind"] = modelID

	paths := make([]string, 0, len(owners))
	for path := range owners {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		value, exists := Lookup(
			result.Resolved,
			path,
		)
		if !exists {
			continue
		}

		record := Provenance{
			Source: "catalog:" + owners[path],
			Value:  value,
		}

		result.Provenance[path] = record
		result.History[path] = []Provenance{
			record,
		}
	}

	for _, overlay := range project {
		if err := validateOverlay(
			overlay,
			modelID,
			"project",
		); err != nil {
			return Resolved{}, err
		}

		mergeResolved(
			result.Resolved,
			overlay.Overrides,
			"",
			"project:"+overlay.ID,
			&result,
		)
	}

	for _, overlay := range host {
		if err := validateOverlay(
			overlay,
			modelID,
			"host",
		); err != nil {
			return Resolved{}, err
		}

		mergeResolved(
			result.Resolved,
			overlay.Overrides,
			"",
			"host:"+overlay.ID,
			&result,
		)
	}

	return result, nil
}
