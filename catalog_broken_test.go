package oddc

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests break a copy of the real catalog in one place and expect the
// registry to refuse it. The repository itself is never changed.

func copyCatalog(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, dir := range []string{"catalog", "evidence"} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			target := filepath.Join(root, path)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			return os.WriteFile(target, data, 0o644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// entityPath finds the copied file of an entity.
func entityPath(t *testing.T, registry *Registry, root, id string) string {
	t.Helper()

	relative, err := filepath.Rel(registry.Root, registry.paths[id])
	if err != nil {
		t.Fatal(err)
	}

	return filepath.Join(root, relative)
}

// editEntity rewrites the copied file of an entity.
func editEntity(t *testing.T, path string, edit func(document map[string]any)) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}

	edit(document)

	data, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func expectRefused(t *testing.T, root, what string) {
	t.Helper()

	if _, err := LoadRegistry(root); err == nil {
		t.Errorf("registry accepted %s", what)
	}
}

func TestBrokenCatalogMissingReference(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		for _, ref := range collectRefs(registry.Entities[id].Data) {
			root := copyCatalog(t)
			if err := os.Remove(entityPath(t, registry, root, ref)); err != nil {
				t.Fatal(err)
			}

			expectRefused(t, root, id+" without "+ref)
		}
	}
}

func TestBrokenCatalogReferenceCycle(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		for _, ref := range collectRefs(registry.Entities[id].Data) {
			root := copyCatalog(t)

			editEntity(t, entityPath(t, registry, root, ref), func(document map[string]any) {
				data := document["data"].(map[string]any)
				data["cycle"] = map[string]any{"ref": id}
			})

			expectRefused(t, root, ref+" referring back to "+id)
		}
	}
}

func TestBrokenCatalogPositionalData(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		root := copyCatalog(t)

		editEntity(t, entityPath(t, registry, root, id), func(document map[string]any) {
			data := document["data"].(map[string]any)
			for key, value := range data {
				data[key] = []any{value}
				break
			}
		})

		expectRefused(t, root, id+" with a list")
	}
}

func TestBrokenCatalogDuplicateID(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		root := copyCatalog(t)
		path := entityPath(t, registry, root, id)

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		copied := strings.TrimSuffix(path, ".json") + "-copy.json"
		if err := os.WriteFile(copied, data, 0o644); err != nil {
			t.Fatal(err)
		}

		expectRefused(t, root, "two files for "+id)
	}
}

// Evidence must name a model, not any other entity.
func TestBrokenCatalogEvidenceForNonModel(t *testing.T) {
	registry := catalog(t)

	err := filepath.WalkDir("evidence", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}

		root := copyCatalog(t)

		var model string
		editEntity(t, filepath.Join(root, path), func(document map[string]any) {
			model = document["deviceId"].(string)
			document["deviceId"] = collectRefs(registry.Entities[model].Data)[0]
		})

		expectRefused(t, root, path+" pointing at a component of "+model)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
