package oddc

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
)

const EntityAPIVersion = "oddc.openjade.de/v2"

// entityID is the form of every stable ID, as in schemas/entity.schema.json.
var entityID = regexp.MustCompile(`^[a-z0-9]+(?:[/-][a-z0-9]+)*$`)

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
