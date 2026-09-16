package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const SchemaVersion = "2.0.0"

type Overlay struct {
	Schema      string         `json:"$schema,omitempty"`
	APIVersion  string         `json:"apiVersion"`
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	TargetModel string         `json:"targetModel"`
	Overrides   map[string]any `json:"overrides"`
}

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

func ReadOverlay(path string) (Overlay, error) {
	file, err := os.Open(path)
	if err != nil {
		return Overlay{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var overlay Overlay
	if err := decoder.Decode(&overlay); err != nil {
		return Overlay{}, fmt.Errorf(
			"decode overlay %s: %w",
			path,
			err,
		)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Overlay{}, fmt.Errorf(
				"%s has trailing JSON",
				path,
			)
		}
		return Overlay{}, err
	}

	if err := validateOverrideObject(
		overlay.Overrides,
		"overrides",
	); err != nil {
		return Overlay{}, fmt.Errorf(
			"%s: %w",
			path,
			err,
		)
	}

	return overlay, nil
}

func validateOverrideObject(value any, path string) error {
	switch typed := value.(type) {
	case []any:
		return fmt.Errorf(
			"%s is positional; overrides must use keyed objects",
			path,
		)

	case map[string]any:
		for key, child := range typed {
			next := key
			if path != "" {
				next = path + "." + key
			}

			if err := validateOverrideObject(
				child,
				next,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateOverlay(
	overlay Overlay,
	modelID string,
	wantKind string,
) error {
	if overlay.APIVersion != EntityAPIVersion {
		return fmt.Errorf(
			"overlay %q apiVersion=%q want=%q",
			overlay.ID,
			overlay.APIVersion,
			EntityAPIVersion,
		)
	}

	if strings.TrimSpace(overlay.ID) == "" {
		return fmt.Errorf("overlay id is required")
	}

	if overlay.Kind != wantKind {
		return fmt.Errorf(
			"overlay %q kind=%q want=%q",
			overlay.ID,
			overlay.Kind,
			wantKind,
		)
	}

	if overlay.TargetModel != modelID {
		return fmt.Errorf(
			"overlay %q targets %q, resolving %q",
			overlay.ID,
			overlay.TargetModel,
			modelID,
		)
	}

	if overlay.Overrides == nil {
		return fmt.Errorf(
			"overlay %q has no overrides",
			overlay.ID,
		)
	}

	return validateOverrideObject(
		overlay.Overrides,
		"overrides",
	)
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

func (r *Registry) traceOwnership(
	value any,
	prefix string,
	sourceID string,
	active map[string]bool,
	owners map[string]string,
) error {
	object, isObject := value.(map[string]any)
	if !isObject {
		if prefix != "" {
			owners[prefix] = sourceID
		}
		return nil
	}

	if ref, ok := object["ref"].(string); ok &&
		strings.TrimSpace(ref) != "" {
		if active[ref] {
			return fmt.Errorf(
				"entity reference cycle through %q",
				ref,
			)
		}

		target, exists := r.Entities[ref]
		if !exists {
			return fmt.Errorf(
				"%q references missing entity %q",
				sourceID,
				ref,
			)
		}

		active[ref] = true

		if err := r.traceOwnership(
			target.Data,
			prefix,
			ref,
			active,
			owners,
		); err != nil {
			return err
		}

		delete(active, ref)

		if prefix != "" {
			owners[prefix+".id"] = ref
			owners[prefix+".name"] = ref
			owners[prefix+".kind"] = ref
		}

		for key, child := range object {
			if key == "ref" {
				continue
			}

			childPath := key
			if prefix != "" {
				childPath = prefix + "." + key
			}

			clearOwners(owners, childPath)

			if err := r.traceOwnership(
				child,
				childPath,
				sourceID,
				active,
				owners,
			); err != nil {
				return err
			}
		}

		return nil
	}

	for key, child := range object {
		childPath := key
		if prefix != "" {
			childPath = prefix + "." + key
		}

		if err := r.traceOwnership(
			child,
			childPath,
			sourceID,
			active,
			owners,
		); err != nil {
			return err
		}
	}

	return nil
}

func clearOwners(
	owners map[string]string,
	path string,
) {
	prefix := path + "."

	for key := range owners {
		if key == path ||
			strings.HasPrefix(key, prefix) {
			delete(owners, key)
		}
	}
}

func mergeResolved(
	dst map[string]any,
	src map[string]any,
	prefix string,
	source string,
	result *Resolved,
) {
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := src[key]

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		incoming, isObject := value.(map[string]any)
		if isObject {
			current, ok := dst[key].(map[string]any)
			if !ok {
				current = map[string]any{}
				dst[key] = current
				clearProvenance(result, path)
			}

			mergeResolved(
				current,
				incoming,
				path,
				source,
				result,
			)
			continue
		}

		clearProvenance(result, path)

		dst[key] = value

		record := Provenance{
			Source: source,
			Value:  value,
		}

		result.Provenance[path] = record
		result.History[path] = append(
			result.History[path],
			record,
		)
	}
}

func clearProvenance(
	result *Resolved,
	path string,
) {
	prefix := path + "."

	for key := range result.Provenance {
		if key == path ||
			strings.HasPrefix(key, prefix) {
			delete(result.Provenance, key)
		}
	}
}

func Lookup(
	root map[string]any,
	path string,
) (any, bool) {
	if path == "" {
		return root, true
	}

	var current any = root

	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}
