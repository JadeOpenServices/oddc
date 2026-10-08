// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
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
func entityPath(t *testing.T, root, id string) string {
	t.Helper()

	return EntityPath(filepath.Join(root, "catalog", "entities"), id)
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
			if err := os.Remove(entityPath(t, root, ref)); err != nil {
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

			editEntity(t, entityPath(t, root, ref), func(document map[string]any) {
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

		editEntity(t, entityPath(t, root, id), func(document map[string]any) {
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
			path := entityPath(t, root, id)

			moved := filepath.Join(filepath.Dir(entityPath(t, root, ref)), filepath.Base(path))
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}

			expectRefused(t, root, id+" stored next to "+ref)
		}
	}
}

// IDs are API: an ID outside the stable form is refused even when the
// file sits at its address.
func TestBrokenCatalogMalformedID(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		root := copyCatalog(t)
		path := entityPath(t, root, id)
		bad := id + "--old"

		editEntity(t, path, func(document map[string]any) {
			document["metadata"].(map[string]any)["id"] = bad
		})
		target := EntityPath(filepath.Join(root, "catalog", "entities"), bad)
		if err := os.Rename(path, target); err != nil {
			t.Fatal(err)
		}

		expectRefused(t, root, "ID "+bad)
	}
}
