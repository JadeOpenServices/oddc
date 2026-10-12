// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// The notes name a part by what it is for, from the path the model places
// it at, with the chip in brackets; list what ODDC makes the machine do
// (policies and quirks) in the owner's words; and never call a part absent
// when the model has none.
func TestDeviceNotesSpeakToTheOwner(t *testing.T) {
	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}
	notes, err := registry.DeviceNotes("model/hp/zbook-x2-g4")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- Kickstand switch",
		"- Card reader: Realtek RTS522A",
		"- Front camera: ",
		"- Fingerprint: Validity VFS495, no Linux driver",
		"- Wi-Fi: ",
		"- USB-C thunderbolt: ",
		"The screen rotates with the tablet.",
		"tablet mode whenever the keyboard is detached",
		"No more interrupt storm",
		"## Not built into this machine",
	} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes lack %q:\n%s", want, notes)
		}
	}
	for _, stale := range []string{"- Realtek RTS522A\n", "- Validity VFS495:", "PME storm on GPE 0x69", "## Not on this machine"} {
		if strings.Contains(notes, stale) {
			t.Errorf("notes still say %q", stale)
		}
	}
	// What ODDC does comes after what works, before what does not.
	works := strings.Index(notes, "## Works\n")
	fixed := strings.Index(notes, "## Works because of ODDC")
	broken := strings.Index(notes, "## Does not work yet")
	if !(works < fixed && fixed < broken) {
		t.Errorf("sections out of order: works %d, fixed %d, broken %d", works, fixed, broken)
	}
}

// A summary is text for people: writing or changing one leaves the closure
// as it is, so it needs no new evidence.
func TestSummariesAreNotPartOfTheClosure(t *testing.T) {
	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}
	const model = "model/hp/zbook-x2-g4"
	before, err := registry.Closure(model)
	if err != nil {
		t.Fatal(err)
	}

	entity := registry.Entities[model]
	policy := entity.Data["policy"].(map[string]any)
	thermal := policy["thermal"].(map[string]any)
	original := thermal["summary"]
	thermal["summary"] = "a different sentence for the owner"
	quirk := registry.Entities["quirk/common/render-on-output-gpu"]
	quirkOriginal := quirk.Data["summary"]
	quirk.Data["summary"] = "another sentence"
	t.Cleanup(func() {
		thermal["summary"] = original
		quirk.Data["summary"] = quirkOriginal
	})

	after, err := registry.Closure(model)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("a summary changed the closure: %s -> %s", before, after)
	}

	// Anything else in the same section still counts.
	daemon := thermal["daemon"]
	thermal["daemon"] = "something-else"
	t.Cleanup(func() { thermal["daemon"] = daemon })
	changed, err := registry.Closure(model)
	if err != nil {
		t.Fatal(err)
	}
	if changed == before {
		t.Error("a policy value did not change the closure")
	}
}
