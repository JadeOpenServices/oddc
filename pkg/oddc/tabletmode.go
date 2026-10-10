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

// Kickstand is a firmware switch that reports whether a model's stand is
// closed (capabilities.kickstandSwitch): the input device of an ACPI
// device, the switch code it sets, and the value that means closed.
type Kickstand struct {
	ACPIID      string `json:"acpiId"`
	Switch      string `json:"switch"`
	ClosedValue int    `json:"closedValue"`
}

// KickstandSwitch reads capabilities.kickstandSwitch of a resolved model.
func KickstandSwitch(resolved map[string]any) (Kickstand, bool) {
	value, ok := Lookup(resolved, "capabilities.kickstandSwitch")
	object, isObject := value.(map[string]any)
	if !ok || !isObject {
		return Kickstand{}, false
	}

	id, _ := object["acpiId"].(string)
	code, _ := object["switch"].(string)
	closed, _ := object["closedValue"].(float64)
	if id == "" || code == "" {
		return Kickstand{}, false
	}

	return Kickstand{ACPIID: id, Switch: code, ClosedValue: int(closed)}, true
}

// TabletMode reports whether a machine is in tablet mode: its stand is
// closed, or none of its detachable keyboards is on its bus. devices are
// what ReadDevices sees.
func TabletMode(keyboards []DetachableKeyboard, devices map[string][]string, standClosed bool) bool {
	if standClosed {
		return true
	}
	for _, keyboard := range keyboards {
		if slices.Contains(devices[keyboard.Bus], keyboard.DeviceID) {
			return false
		}
	}

	return true
}
