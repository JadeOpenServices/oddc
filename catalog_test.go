package oddc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryCatalogLoads(t *testing.T) {
	root, err := filepath.Abs(
		filepath.Join("..", "..", "oddc"),
	)
	if err != nil {
		t.Fatal(err)
	}

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog.Documents) == 0 {
		t.Fatal("catalog contains no documents")
	}
}

func TestFrameworkResolutionAndOverridePrecedence(t *testing.T) {
	root, err := filepath.Abs(
		filepath.Join("..", "..", "oddc"),
	)
	if err != nil {
		t.Fatal(err)
	}

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	project := Overlay{
		SchemaVersion: SchemaVersion,
		ID:            "project/test",
		Kind:          "project",
		TargetDevice:  "framework-laptop-13-amd-ryzen-7040",
		Blocks: map[string]any{
			"thermal": map[string]any{
				"fan-control": map[string]any{
					"policy": map[string]any{
						"thermalEnterC": 85,
					},
				},
			},
		},
	}

	host := Overlay{
		SchemaVersion: SchemaVersion,
		ID:            "host/test",
		Kind:          "host",
		TargetDevice:  "framework-laptop-13-amd-ryzen-7040",
		Blocks: map[string]any{
			"thermal": map[string]any{
				"fan-control": map[string]any{
					"policy": map[string]any{
						"thermalEnterC": 87,
					},
				},
			},
		},
	}

	resolved, err := catalog.Resolve(
		"framework-laptop-13-amd-ryzen-7040",
		[]Overlay{project},
		[]Overlay{host},
	)
	if err != nil {
		t.Fatal(err)
	}

	const path = "thermal.fan-control.policy.thermalEnterC"

	value, exists := Lookup(resolved.Blocks, path)
	if !exists {
		t.Fatalf("%s missing", path)
	}

	if value != 87 {
		t.Fatalf("%s=%v want=87", path, value)
	}

	current := resolved.Provenance[path]
	if current.Source != "host:host/test" {
		t.Fatalf(
			"%s source=%q want=%q",
			path,
			current.Source,
			"host:host/test",
		)
	}

	history := resolved.History[path]
	if len(history) != 3 {
		t.Fatalf(
			"%s history entries=%d want=3",
			path,
			len(history),
		)
	}
}

func TestOverrideableBlocksRejectArrays(t *testing.T) {
	root := t.TempDir()

	path := filepath.Join(
		root,
		"catalog",
		"devices",
		"bad.json",
	)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	data := []byte(`{
  "schemaVersion": "2.0.0",
  "id": "bad-device",
  "kind": "device",
  "blocks": {
    "bad": [1, 2, 3]
  }
}
`)

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(root); err == nil {
		t.Fatal("array inside overrideable blocks was accepted")
	}
}
