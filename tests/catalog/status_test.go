// SPDX-License-Identifier: GPL-3.0-or-later

package catalog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// status --since passes when no model changed, and fails for a model whose
// closure changed without evidence on the new one.
func TestStatusSince(t *testing.T) {
	root := fixture.GitCatalog(t)
	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.RunStatus(registry, []string{"status", "--since", "base"}); err != nil {
		t.Fatalf("unchanged: %v", err)
	}

	model := changeModel(t, root, registry)
	if registry, err = oddc.LoadRegistry(root); err != nil {
		t.Fatal(err)
	}
	err = catalog.RunStatus(registry, []string{"status", "--since", "base", "--json"})
	if err == nil || !strings.Contains(err.Error(), model+" is ") || strings.Contains(err.Error(), model+" is verified") {
		t.Fatalf("%s changed: %v", model, err)
	}
}

// changeModel changes the data of the first model of registry, at root, so
// that its closure changes, and returns it.
func changeModel(t *testing.T, root string, registry *oddc.Registry) string {
	t.Helper()

	var models []string
	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	model := models[0]

	// The model's own data changes, as in TestVerify.
	path := oddc.EntityPath(filepath.Join(root, "catalog", "entities"), model)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	data := document["data"].(map[string]any)
	keys := []string{}
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	delete(data, keys[0])
	if raw, err = json.Marshal(document); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, path, raw)

	return model
}

// status --verified fails for a model that is not verified, and names it;
// without --verified status only reports.
func TestStatusVerified(t *testing.T) {
	root := fixture.GitCatalog(t)
	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	model := changeModel(t, root, registry)
	if registry, err = oddc.LoadRegistry(root); err != nil {
		t.Fatal(err)
	}

	err = catalog.RunStatus(registry, []string{"status", "--verified"})
	if err == nil || !strings.Contains(err.Error(), "not verified: ") || !strings.Contains(err.Error(), model+" is ") {
		t.Errorf("%s changed: %v", model, err)
	}
	if err := catalog.RunStatus(registry, []string{"status", "--device", model}); err != nil {
		t.Errorf("%s without --verified: %v", model, err)
	}
}
