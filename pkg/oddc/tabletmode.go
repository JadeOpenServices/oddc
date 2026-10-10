// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"slices"
	"strings"
)

// DetachableKeyboard is a keyboard a model says leaves its bus when it is
// detached (detachSignal "bus-presence"), so its presence tells laptop
// from tablet mode.
type DetachableKeyboard struct {
	Path     string
	Bus      string
	DeviceID string
}

// TabletModeKeyboards lists the detachable keyboards of a resolved model
// whose presence signals tablet mode. A model with a working tablet mode
// switch (capabilities.tabletModeSwitch true) has none: the switch is the
// signal there.
func TabletModeKeyboards(resolved map[string]any) []DetachableKeyboard {
	if working, _ := Lookup(resolved, "capabilities.tabletModeSwitch"); working == true {
		return nil
	}

	var found []DetachableKeyboard
	for _, component := range modelComponents(resolved) {
		if !strings.HasPrefix(component.Path, "hardware.input.keyboard.") {
			continue
		}
		object := componentAt(resolved, component.Path)
		if object["attachment"] != "detachable" || object["detachSignal"] != "bus-presence" {
			continue
		}
		found = append(found, DetachableKeyboard{
			Path:     component.Path,
			Bus:      factsBus(component.Bus),
			DeviceID: strings.ToLower(component.DeviceID),
		})
	}

	return found
}

// TabletMode reports whether a machine is in tablet mode: none of its
// detachable keyboards is on its bus. devices are what ReadDevices sees.
func TabletMode(keyboards []DetachableKeyboard, devices map[string][]string) bool {
	for _, keyboard := range keyboards {
		if slices.Contains(devices[keyboard.Bus], keyboard.DeviceID) {
			return false
		}
	}

	return true
}
