package oddc_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Every test runs over the real catalog and takes its expectations from
// the catalog and evidence. Device facts are never repeated here.

func catalog(t *testing.T) *Registry {
	t.Helper()

	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}

	return registry
}

func models(t *testing.T, registry *Registry) []string {
	t.Helper()

	ids := modelIDs(registry)
	sort.Strings(ids)

	if len(ids) == 0 {
		t.Fatal("catalog has no models")
	}

	return ids
}

// leaves returns every scalar value below root by path.
func leaves(root map[string]any, prefix string, result map[string]any) {
	for key, value := range root {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if object, ok := value.(map[string]any); ok {
			leaves(object, path, result)
			continue
		}

		result[path] = value
	}
}

func leavesOf(root map[string]any) map[string]any {
	result := map[string]any{}
	leaves(root, "", result)
	return result
}

func TestCatalogEveryModelResolves(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		if _, err := registry.ResolveModel(id, nil, nil); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
}

// Every resolved value names its owner, and the owner's own file holds it.
func TestCatalogEveryValueHasItsOwner(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		resolved, err := registry.ResolveModel(id, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		for path, value := range leavesOf(resolved.Resolved) {
			record, exists := resolved.Provenance[path]
			if !exists {
				t.Errorf("%s: %s has no owner", id, path)
				continue
			}

			owner, exists := registry.Entities[strings.TrimPrefix(record.Source, "catalog:")]
			if !exists {
				t.Errorf("%s: %s owned by unknown %q", id, path, record.Source)
				continue
			}

			if !ownerHolds(owner, path, value) {
				t.Errorf("%s: %s=%v not in owner %s", id, path, value, owner.Metadata.ID)
			}
		}
	}
}

// ownerHolds reports whether the owner's data holds value at a suffix of
// path, or path ends in the identity the resolver adds for a reference.
func ownerHolds(owner Entity, path string, value any) bool {
	parts := strings.Split(path, ".")

	switch parts[len(parts)-1] {
	case "id":
		if value == owner.Metadata.ID {
			return true
		}
	case "name":
		if value == owner.Metadata.Name {
			return true
		}
	case "kind":
		if value == owner.Kind {
			return true
		}
	}

	for i := range parts {
		if held, ok := Lookup(owner.Data, strings.Join(parts[i:], ".")); ok &&
			fmt.Sprint(held) == fmt.Sprint(value) {
			return true
		}
	}

	return false
}

func TestCatalogValidates(t *testing.T) {
	registry := catalog(t)

	result := Validate(".")
	if !result.Valid || len(result.Errors) != 0 {
		t.Fatalf("catalog invalid: %+v", result)
	}
	if result.Entities != len(registry.Entities) {
		t.Errorf("entities = %d, want %d", result.Entities, len(registry.Entities))
	}
	if result.Evidence != len(evidenceFiles(t)) {
		t.Errorf("evidence = %d, want %d", result.Evidence, len(evidenceFiles(t)))
	}
}

// The index names every entity at the file that holds it, and every
// evidence record under its model.
func TestCatalogIndex(t *testing.T) {
	registry := catalog(t)

	index, err := registry.Index("test")
	if err != nil {
		t.Fatal(err)
	}

	if len(index.Entities) != len(registry.Entities) {
		t.Fatalf("index has %d entities, want %d", len(index.Entities), len(registry.Entities))
	}

	var evidence []string
	for id, entry := range index.Entities {
		entity, err := decodeEntity(filepath.FromSlash(entry.Path))
		if err != nil {
			t.Fatal(err)
		}
		if entity.Metadata.ID != id || entity.Kind != entry.Kind {
			t.Errorf("%s: index points at %s holding %s", id, entry.Path, entity.Metadata.ID)
		}
		for _, file := range entry.Evidence {
			evidence = append(evidence, filepath.FromSlash(file))
		}
	}

	sort.Strings(evidence)
	want := evidenceFiles(t)
	sort.Strings(want)
	if strings.Join(evidence, ",") != strings.Join(want, ",") {
		t.Errorf("index evidence %v, want %v", evidence, want)
	}
}
