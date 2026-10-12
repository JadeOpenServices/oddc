// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// A component whose entity marks it unusable on Linux is required to be
// "unsupported" in hardware-validated evidence, with the reason its entity
// gives; every other component stays "pass".
func TestUnsupportedComponents(t *testing.T) {
	registry, models := fixture.Catalog(t)
	found := 0

	for _, model := range models {
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		required, err := registry.Requirements(model)
		if err != nil {
			t.Fatal(err)
		}

		unsupported := map[string]bool{}
		for _, part := range UnsupportedComponents(resolved.Resolved) {
			found++
			unsupported[part.Path] = true

			entity := registry.Entities[part.ID].Data
			if status, _ := Lookup(entity, "support.linux.status"); status != "unsupported" {
				t.Errorf("%s: %s reported unsupported, its entity says %v", model, part.ID, status)
			}
			if reason, _ := Lookup(entity, "support.linux.reason"); part.Reason == "" || part.Reason != reason {
				t.Errorf("%s: %s reason %q, entity %v", model, part.ID, part.Reason, reason)
			}
			if required[part.Path] != "unsupported" {
				t.Errorf("%s: %s required %q, want unsupported", model, part.Path, required[part.Path])
			}
		}

		for path, want := range required {
			if strings.HasPrefix(path, "hardware.") && !unsupported[path] && want != "pass" {
				t.Errorf("%s: %s required %q, want pass", model, path, want)
			}
		}
	}

	if found == 0 {
		t.Fatal("no model has a component marked unusable on Linux")
	}
}
