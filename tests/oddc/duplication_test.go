// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Each test copies the real catalog, puts one value in a second place and
// expects validation to name the problem. The repository is never changed.

func expectDuplication(t *testing.T, root, what, want string) {
	t.Helper()

	result := Validate(root)
	if result.Valid {
		t.Fatalf("validation passed %s", what)
	}
	for _, problem := range result.Errors {
		if strings.Contains(problem, want) {
			return
		}
	}
	t.Errorf("%s: no error mentions %q: %v", what, want, result.Errors)
}

// firstOfKind returns the alphabetically first entity of a kind.
func firstOfKind(t *testing.T, registry *Registry, accept func(Entity) bool) string {
	t.Helper()

	first := ""
	for id, entity := range registry.Entities {
		if accept(entity) && (first == "" || id < first) {
			first = id
		}
	}
	if first == "" {
		t.Skip("the catalog has no such entity")
	}
	return first
}

func TestRealCatalogHasNoDuplication(t *testing.T) {
	if problems := catalog(t).Duplication(); len(problems) != 0 {
		t.Errorf("the catalog repeats itself:\n%s", strings.Join(problems, "\n"))
	}
}

// A second quirk entity with the same data: two owners of one behaviour.
func TestDuplicateQuirkData(t *testing.T) {
	registry := catalog(t)
	quirk := firstOfKind(t, registry, func(e Entity) bool { return e.Kind == "Quirk" })

	root := copyCatalog(t)
	original := entityPath(t, root, quirk)
	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	copyID := quirk + "-copy"
	copyPath := filepath.Join(filepath.Dir(original), filepath.Base(copyID)+".json")
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	editEntity(t, copyPath, func(document map[string]any) {
		document["metadata"].(map[string]any)["id"] = copyID
	})

	expectDuplication(t, root, "a copied quirk", "hold the same quirk data")
}

// A model writing quirk data next to its reference.
func TestModelQuirkCarriesData(t *testing.T) {
	registry := catalog(t)
	model := firstOfKind(t, registry, func(e Entity) bool {
		quirks, _ := e.Data["quirks"].(map[string]any)
		return e.Kind == "DeviceModel" && len(quirks) > 0
	})

	root := copyCatalog(t)
	editEntity(t, entityPath(t, root, model), func(document map[string]any) {
		quirks := document["data"].(map[string]any)["quirks"].(map[string]any)
		for _, value := range quirks {
			value.(map[string]any)["extra"] = true
			return
		}
	})

	expectDuplication(t, root, "a model's own quirk data", "only references a quirk")
}

// A model describing a device itself instead of referencing its component.
func TestModelCopiesComponent(t *testing.T) {
	registry := catalog(t)
	model := firstOfKind(t, registry, func(e Entity) bool { return e.Kind == "DeviceModel" })
	component := firstOfKind(t, registry, func(e Entity) bool {
		_, bus := e.Data["bus"].(string)
		_, device := e.Data["deviceId"].(string)
		return strings.HasSuffix(e.Kind, "Component") && bus && device
	})

	root := copyCatalog(t)
	editEntity(t, entityPath(t, root, model), func(document map[string]any) {
		hardware := document["data"].(map[string]any)["hardware"].(map[string]any)
		hardware["copied"] = map[string]any{
			"bus":      registry.Entities[component].Data["bus"],
			"deviceId": registry.Entities[component].Data["deviceId"],
		}
	})

	expectDuplication(t, root, "a component copied into a model", "make it a component entity")
}

// Two component entities claiming one device.
func TestTwoComponentsOneDevice(t *testing.T) {
	registry := catalog(t)
	component := firstOfKind(t, registry, func(e Entity) bool {
		_, bus := e.Data["bus"].(string)
		_, device := e.Data["deviceId"].(string)
		return strings.HasSuffix(e.Kind, "Component") && bus && device
	})

	root := copyCatalog(t)
	original := entityPath(t, root, component)
	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	copyID := component + "-copy"
	copyPath := filepath.Join(filepath.Dir(original), filepath.Base(copyID)+".json")
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	editEntity(t, copyPath, func(document map[string]any) {
		document["metadata"].(map[string]any)["id"] = copyID
	})

	expectDuplication(t, root, "a second component for one device", "one component entity owns a device")
}
