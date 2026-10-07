// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"path/filepath"
	"sort"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

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

		if unnamed := verifyCopy(t, path, func(root string, record map[string]any) {
			onClosure(root, record)
			delete(record, "closure")
		}); unnamed.Status != Unverified {
			t.Errorf("%s without a closure: %+v", path, unnamed)
		}
	}
}
