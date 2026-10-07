// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func decodeEntity(path string) (Entity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entity{}, err
	}

	return decodeEntityData(path, data)
}

// decodeEntityData decodes one entity document; name labels errors.
func decodeEntityData(name string, data []byte) (Entity, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var entity Entity

	if err := decoder.Decode(&entity); err != nil {
		return Entity{}, fmt.Errorf("decode %s: %w", name, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Entity{}, fmt.Errorf("%s has trailing JSON", name)
		}
		return Entity{}, err
	}

	return entity, nil
}

func validateEntity(entity Entity) error {
	if entity.APIVersion != EntityAPIVersion {
		return fmt.Errorf(
			"apiVersion=%q want=%q",
			entity.APIVersion,
			EntityAPIVersion,
		)
	}

	if strings.TrimSpace(entity.Kind) == "" {
		return fmt.Errorf("kind is required")
	}

	if strings.TrimSpace(entity.Metadata.ID) == "" {
		return fmt.Errorf("metadata.id is required")
	}

	if !entityID.MatchString(entity.Metadata.ID) {
		return fmt.Errorf(
			"metadata.id %q is not lowercase kebab-case segments separated by /",
			entity.Metadata.ID,
		)
	}

	if strings.TrimSpace(entity.Metadata.Name) == "" {
		return fmt.Errorf("metadata.name is required")
	}

	if entity.Data == nil {
		return fmt.Errorf("data is required")
	}

	return rejectEntityArrays(entity.Data, "data")
}

func rejectEntityArrays(value any, path string) error {
	switch typed := value.(type) {
	case []any:
		return fmt.Errorf(
			"%s is positional; canonical ODDC entities must use keyed objects",
			path,
		)

	case map[string]any:
		for key, child := range typed {
			next := key
			if path != "" {
				next = path + "." + key
			}

			if err := rejectEntityArrays(child, next); err != nil {
				return err
			}
		}
	}

	return nil
}

func collectRefs(value any) []string {
	set := map[string]struct{}{}

	var walk func(any)

	walk = func(current any) {
		object, ok := current.(map[string]any)
		if !ok {
			return
		}

		if ref, ok := object["ref"].(string); ok && ref != "" {
			set[ref] = struct{}{}
		}

		for _, child := range object {
			walk(child)
		}
	}

	walk(value)

	refs := make([]string, 0, len(set))
	for ref := range set {
		refs = append(refs, ref)
	}

	sort.Strings(refs)

	return refs
}
