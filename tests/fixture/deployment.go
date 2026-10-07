package fixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Deployment lays out a model as the NixOS module deploys it: the catalog,
// its evidence, resolved.json and, when set, a host overlay that keeps the
// model's own canonical values.
func Deployment(t *testing.T, registry *oddc.Registry, model string, overlay bool) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"catalog", "evidence"} {
		if err := os.Symlink(filepath.Join(Repository, dir), filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}

	resolved, err := registry.ResolveModel(model, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(resolved.Resolved)
	if err != nil {
		t.Fatal(err)
	}
	Write(t, filepath.Join(root, "resolved.json"), data)

	if overlay {
		hardware, _ := oddc.Lookup(resolved.Resolved, "hardware")
		data, err := json.Marshal(oddc.Overlay{
			APIVersion:  oddc.EntityAPIVersion,
			ID:          "host/" + model,
			Kind:        "host",
			TargetModel: model,
			Overrides:   map[string]any{"hardware": hardware},
		})
		if err != nil {
			t.Fatal(err)
		}
		Write(t, filepath.Join(root, "host-overlay.json"), data)
	}

	return root
}

// FactsFile writes the facts a machine of a model reports.
func FactsFile(t *testing.T, registry *oddc.Registry, model string) string {
	t.Helper()

	facts, err := registry.ModelFacts(model)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "facts.json")
	Write(t, file, data)

	return file
}
