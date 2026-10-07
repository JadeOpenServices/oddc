package oddc_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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

// Evidence names drivers only for components of its model.
func TestBrokenCatalogEvidenceDriverForOtherComponent(t *testing.T) {
	registry := catalog(t)

	for _, path := range evidenceFiles(t) {
		root := copyCatalog(t)

		var other string
		editEntity(t, filepath.Join(root, path), func(document map[string]any) {
			own := collectRefs(registry.Entities[document["deviceId"].(string)].Data)
			for _, model := range models(t, registry) {
				for _, ref := range collectRefs(registry.Entities[model].Data) {
					if other == "" && registry.Entities[ref].Data["driver"] != nil && !slices.Contains(own, ref) {
						other = ref
					}
				}
			}
			document["drivers"] = map[string]any{other: []any{registry.Entities[other].Data["driver"]}}
		})
		if other == "" {
			t.Skip("every component with a driver belongs to every model")
		}

		expectRefused(t, root, path+" with drivers for "+other)
	}
}
