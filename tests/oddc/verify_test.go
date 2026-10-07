// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// proof fills a record with every result and driver that proves model
// hardware-validated.
func proof(t *testing.T, registry *Registry, model string, record map[string]any) {
	t.Helper()

	required, err := registry.Requirements(model)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.ResolveEntity(model)
	if err != nil {
		t.Fatal(err)
	}

	results, drivers := map[string]any{}, map[string]any{}
	for name, want := range required {
		results[name] = want
		if !strings.HasPrefix(name, "hardware.") {
			continue
		}
		component := resolved.Resolved
		for _, key := range strings.Split(name, ".") {
			component = component[key].(map[string]any)
		}
		if driver, ok := component["driver"].(string); ok {
			drivers[component["id"].(string)] = []any{driver}
		}
	}
	record["results"], record["drivers"] = results, drivers
}

// verifyCopy loads a copy of the catalog in which edit changed the
// evidence record at path, and verifies its model.
func verifyCopy(t *testing.T, path string, edit func(root string, record map[string]any)) Verification {
	t.Helper()

	root := copyCatalog(t)
	var model string
	editEntity(t, filepath.Join(root, path), func(record map[string]any) {
		model = record["deviceId"].(string)
		edit(root, record)
	})

	registry, err := LoadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := registry.Verify(model)
	if err != nil {
		t.Fatal(err)
	}

	return status
}

// A model is verified by passing evidence recorded on its current closure,
// changed once the closure moves on, and unverified without passing
// evidence that names a closure.
func TestVerify(t *testing.T) {
	registry := catalog(t)

	for _, path := range evidenceFiles(t) {
		var id, model string
		onClosure := func(root string, record map[string]any) {
			id, model = record["id"].(string), record["deviceId"].(string)
			closure, err := registry.Closure(model)
			if err != nil {
				t.Fatal(err)
			}
			record["closure"] = closure
			record["status"] = "hardware-validated"
			proof(t, registry, model, record)
		}

		if status := verifyCopy(t, path, onClosure); status.Status != Verified || status.Evidence.ID != id {
			t.Errorf("%s on its closure: %+v", path, status)
		}

		changed := verifyCopy(t, path, func(root string, record map[string]any) {
			onClosure(root, record)
			editEntity(t, entityPath(t, root, model), func(document map[string]any) {
				// The model's own data changes; its name is not part of it.
				data := document["data"].(map[string]any)
				keys := []string{}
				for key := range data {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				delete(data, keys[0])
			})
		})
		if changed.Status != Changed || changed.Evidence.ID != id {
			t.Errorf("%s after its model changed: %+v", path, changed)
		}

		documented := verifyCopy(t, path, func(root string, record map[string]any) {
			onClosure(root, record)
			record["status"] = "documented"
		})
		if documented.Status != Unverified || documented.Evidence.ID != id {
			t.Errorf("%s documented only: %+v", path, documented)
		}

		// Passing evidence on the current closure that misses one
		// requirement does not validate.
		root := copyCatalog(t)
		editEntity(t, filepath.Join(root, path), func(record map[string]any) {
			onClosure(root, record)
			delete(record["results"].(map[string]any), "identity")
		})
		if _, err := LoadRegistry(root); err == nil || !strings.Contains(err.Error(), "result identity: missing") {
			t.Errorf("%s without identity: %v", path, err)
		}

		if unnamed := verifyCopy(t, path, func(root string, record map[string]any) {
			onClosure(root, record)
			delete(record, "closure")
		}); unnamed.Status != Unverified {
			t.Errorf("%s without a closure: %+v", path, unnamed)
		}
	}
}

// A kernel-ranged quirk the deployment did not apply is proven
// not-affected; a quirk without a kernel range is always applied.
func TestVerifyInactiveQuirks(t *testing.T) {
	registry := catalog(t)

	tested := false
	for _, path := range evidenceFiles(t) {
		var ranged, always string
		onClosure := func(root string, record map[string]any) {
			model := record["deviceId"].(string)
			resolved, err := registry.ResolveEntity(model)
			if err != nil {
				t.Fatal(err)
			}
			quirks, _ := resolved.Resolved["quirks"].(map[string]any)
			for _, key := range sortedKeys(quirks) {
				if _, ok := quirks[key].(map[string]any)["affected"]; ok && ranged == "" {
					ranged = key
				} else if !ok && always == "" {
					always = key
				}
			}
			closure, err := registry.Closure(model)
			if err != nil {
				t.Fatal(err)
			}
			record["closure"] = closure
			record["status"] = "hardware-validated"
			proof(t, registry, model, record)
		}

		load := func(edit func(record map[string]any)) error {
			root := copyCatalog(t)
			editEntity(t, filepath.Join(root, path), func(record map[string]any) {
				onClosure(root, record)
				edit(record)
			})
			_, err := LoadRegistry(root)
			return err
		}

		if err := load(func(map[string]any) {}); err != nil {
			t.Fatal(err)
		}
		if ranged == "" || always == "" {
			continue
		}
		tested = true

		err := load(func(record map[string]any) { record["inactiveQuirks"] = []any{ranged} })
		if want := "result quirks." + ranged + ": pass, want not-affected"; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s with %s inactive but passing: %v, want %q", path, ranged, err, want)
		}

		if status := verifyCopy(t, path, func(root string, record map[string]any) {
			onClosure(root, record)
			record["inactiveQuirks"] = []any{ranged}
			record["results"].(map[string]any)["quirks."+ranged] = "not-affected"
		}); status.Status != Verified {
			t.Errorf("%s with %s inactive and not affected: %+v", path, ranged, status)
		}

		err = load(func(record map[string]any) { record["inactiveQuirks"] = []any{always} })
		if want := "quirk " + always + ": not kernel-ranged"; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s with %s inactive: %v, want %q", path, always, err, want)
		}
	}
	if !tested {
		t.Fatal("no evidence is for a model with a kernel-ranged and an unranged quirk")
	}
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
