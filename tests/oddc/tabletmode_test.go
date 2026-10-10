// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"slices"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// A model whose detachable keyboard signals tablet mode by leaving its bus
// is in laptop mode while its own devices are present and in tablet mode
// once that keyboard is gone; a model with a working tablet mode switch
// has no such keyboard.
func TestTabletModeFromKeyboardPresence(t *testing.T) {
	registry, models := fixture.Catalog(t)
	found := 0

	for _, model := range models {
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		keyboards := TabletModeKeyboards(resolved.Resolved)
		if switchWorks, _ := Lookup(resolved.Resolved, "capabilities.tabletModeSwitch"); switchWorks == true && len(keyboards) != 0 {
			t.Errorf("%s has a working switch and still keyboards for tablet mode", model)
		}
		if len(keyboards) == 0 {
			continue
		}
		found++

		facts, err := registry.ModelFacts(model)
		if err != nil {
			t.Fatal(err)
		}
		if TabletMode(keyboards, facts.Devices) {
			t.Errorf("%s: tablet mode with its keyboard present", model)
		}

		detached := map[string][]string{}
		for bus, ids := range facts.Devices {
			detached[bus] = slices.DeleteFunc(slices.Clone(ids), func(id string) bool {
				return slices.ContainsFunc(keyboards, func(k DetachableKeyboard) bool { return k.Bus == bus && k.DeviceID == id })
			})
		}
		if !TabletMode(keyboards, detached) {
			t.Errorf("%s: laptop mode with its keyboard detached", model)
		}
	}

	if found == 0 {
		t.Fatal("no model signals tablet mode through a detachable keyboard")
	}
}
