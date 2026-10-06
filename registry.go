package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const EntityAPIVersion = "oddc.openjade.de/v2"

type EntityMetadata struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Entity struct {
	Schema     string         `json:"$schema,omitempty"`
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   EntityMetadata `json:"metadata"`
	Data       map[string]any `json:"data"`
	Sources    map[string]any `json:"sources,omitempty"`
}

type Registry struct {
	Root     string
	Entities map[string]Entity
	paths    map[string]string
}

type ResolvedEntity struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Resolved   map[string]any `json:"resolved"`
	References []string       `json:"references"`
}

func LoadRegistry(root string) (*Registry, error) {
	registry := &Registry{
		Root:     root,
		Entities: map[string]Entity{},
		paths:    map[string]string{},
	}

	entityRoot := filepath.Join(root, "catalog", "entities")

	err := filepath.WalkDir(
		entityRoot,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}

			entity, err := decodeEntity(path)
			if err != nil {
				return err
			}

			if err := validateEntity(entity); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}

			// An entity's ID is its address: catalog/entities/<id>.json.
			if want := EntityPath(entityRoot, entity.Metadata.ID); path != want {
				return fmt.Errorf(
					"%s holds entity %q, which belongs in %s",
					path,
					entity.Metadata.ID,
					want,
				)
			}

			if previous, exists := registry.paths[entity.Metadata.ID]; exists {
				return fmt.Errorf(
					"duplicate entity id %q in %s and %s",
					entity.Metadata.ID,
					previous,
					path,
				)
			}

			registry.Entities[entity.Metadata.ID] = entity
			registry.paths[entity.Metadata.ID] = path

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("load entity registry: %w", err)
	}

	for id, entity := range registry.Entities {
		refs := collectRefs(entity.Data)

		for _, ref := range refs {
			if _, exists := registry.Entities[ref]; !exists {
				return nil, fmt.Errorf(
					"%q references missing entity %q",
					id,
					ref,
				)
			}
		}
	}

	for id := range registry.Entities {
		if _, err := registry.ResolveEntity(id); err != nil {
			return nil, err
		}
	}

	if err := registry.validateEvidence(); err != nil {
		return nil, err
	}

	return registry, nil
}

// EntityPath returns where the entity with this ID lives below root.
func EntityPath(root, id string) string {
	return filepath.Join(root, filepath.FromSlash(id)+".json")
}

func decodeEntity(path string) (Entity, error) {
	file, err := os.Open(path)
	if err != nil {
		return Entity{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var entity Entity

	if err := decoder.Decode(&entity); err != nil {
		return Entity{}, fmt.Errorf("decode %s: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Entity{}, fmt.Errorf("%s has trailing JSON", path)
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

func (r *Registry) ResolveEntity(id string) (ResolvedEntity, error) {
	entity, exists := r.Entities[id]
	if !exists {
		return ResolvedEntity{}, fmt.Errorf("unknown entity %q", id)
	}

	active := map[string]bool{}

	resolved, err := r.expandObject(entity.Data, active, id)
	if err != nil {
		return ResolvedEntity{}, err
	}

	return ResolvedEntity{
		ID:         entity.Metadata.ID,
		Name:       entity.Metadata.Name,
		Kind:       entity.Kind,
		Resolved:   resolved,
		References: collectRefs(entity.Data),
	}, nil
}

func (r *Registry) expandObject(
	object map[string]any,
	active map[string]bool,
	context string,
) (map[string]any, error) {
	if ref, ok := object["ref"].(string); ok && ref != "" {
		if active[ref] {
			return nil, fmt.Errorf(
				"entity reference cycle while resolving %q through %q",
				context,
				ref,
			)
		}

		target, exists := r.Entities[ref]
		if !exists {
			return nil, fmt.Errorf(
				"%q references missing entity %q",
				context,
				ref,
			)
		}

		active[ref] = true

		base, err := r.expandObject(target.Data, active, ref)
		if err != nil {
			return nil, err
		}

		delete(active, ref)

		result := cloneObject(base)
		result["id"] = target.Metadata.ID
		result["name"] = target.Metadata.Name

		for key, value := range object {
			if key == "ref" {
				continue
			}

			expanded, err := r.expandValue(value, active, context)
			if err != nil {
				return nil, err
			}

			result[key] = expanded
		}

		return result, nil
	}

	result := make(map[string]any, len(object))

	for key, value := range object {
		expanded, err := r.expandValue(value, active, context)
		if err != nil {
			return nil, err
		}

		result[key] = expanded
	}

	return result, nil
}

func (r *Registry) expandValue(
	value any,
	active map[string]bool,
	context string,
) (any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	return r.expandObject(object, active, context)
}

func cloneObject(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))

	for key, value := range source {
		object, ok := value.(map[string]any)
		if ok {
			result[key] = cloneObject(object)
			continue
		}

		result[key] = value
	}

	return result
}
