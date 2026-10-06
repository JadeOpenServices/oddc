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

	if result := Validate(root); result.Valid || len(result.Errors) == 0 {
		t.Errorf("validation passed %s: %+v", what, result)
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

// An entity lives at the path its ID names; a moved file is refused.
func TestBrokenCatalogMisplacedEntity(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		for _, ref := range collectRefs(registry.Entities[id].Data) {
			root := copyCatalog(t)
			path := entityPath(t, registry, root, id)

			moved := filepath.Join(filepath.Dir(entityPath(t, registry, root, ref)), filepath.Base(path))
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}

			expectRefused(t, root, id+" stored next to "+ref)
		}
	}
}

// evidenceFiles returns every real evidence record.
func evidenceFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir("evidence", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(files) == 0 {
		t.Fatal("catalog has no evidence")
	}

	return files
}

// Evidence lives below the model it is about.
func TestBrokenCatalogMisplacedEvidence(t *testing.T) {
	registry := catalog(t)

	for _, path := range evidenceFiles(t) {
		for _, other := range models(t, registry) {
			if strings.HasPrefix(path, filepath.Join("evidence", filepath.FromSlash(other))+string(filepath.Separator)) {
				continue
			}

			root := copyCatalog(t)
			target := filepath.Join(root, "evidence", filepath.FromSlash(other), filepath.Base(path))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(root, path), target); err != nil {
				t.Fatal(err)
			}

			expectRefused(t, root, path+" stored below "+other)
		}
	}
}

// Evidence must name a model, not any other entity, even when stored
// below that entity.
func TestBrokenCatalogEvidenceForNonModel(t *testing.T) {
	registry := catalog(t)

	for _, path := range evidenceFiles(t) {
		root := copyCatalog(t)

		var model, component string
		editEntity(t, filepath.Join(root, path), func(document map[string]any) {
			model = document["deviceId"].(string)
			component = collectRefs(registry.Entities[model].Data)[0]
			document["deviceId"] = component
		})

		target := filepath.Join(root, "evidence", filepath.FromSlash(component), filepath.Base(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(root, path), target); err != nil {
			t.Fatal(err)
		}

		expectRefused(t, root, path+" pointing at "+component+" of "+model)
	}
}
