// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"encoding/json"
	"fmt"
)

// QuickKeys is what a resolved model says about its Quick Keys: the
// component at hardware.input.quickKeys.primary, the key each button
// sends per preset (policy.input.quickKeys.keymap) and the key the preset
// switch sends when it selects a preset (policy.input.quickKeys.presetKeys).
type QuickKeys struct {
	DeviceID   string                       `json:"deviceId"`
	Protocol   QuickKeysProtocol            `json:"protocol"`
	Keymap     map[string]map[string]string `json:"keymap"`
	PresetKeys map[string]string            `json:"presetKeys"`
}

// QuickKeysProtocol is the input report of a Quick Keys device: one report
// ID and length, a byte with one bit per button, a byte naming the active
// preset, and bytes that always hold a fixed value.
type QuickKeysProtocol struct {
	ReportID         int                        `json:"reportId"`
	ReportLength     int                        `json:"reportLength"`
	ButtonByteOffset int                        `json:"buttonByteOffset"`
	PresetByteOffset int                        `json:"presetByteOffset"`
	Framing          map[string]int             `json:"framing"`
	Buttons          map[string]QuickKeysButton `json:"buttons"`
	Presets          map[string]QuickKeysPreset `json:"presets"`
}

type QuickKeysButton struct {
	Raw          int  `json:"raw"`
	PresetSwitch bool `json:"presetSwitch"`
}

type QuickKeysPreset struct {
	Raw int `json:"raw"`
}

// QuickKeysFrom reads the Quick Keys of a resolved view. A view without a
// mappable Quick Keys component has none.
func QuickKeysFrom(resolved map[string]any) (QuickKeys, bool, error) {
	component, ok := Lookup(resolved, "hardware.input.quickKeys.primary")
	if !ok {
		return QuickKeys{}, false, nil
	}
	if mappable, _ := Lookup(resolved, "hardware.input.quickKeys.primary.access.mappable"); mappable != true {
		return QuickKeys{}, false, nil
	}
	keymap, _ := Lookup(resolved, "policy.input.quickKeys.keymap")
	presetKeys, _ := Lookup(resolved, "policy.input.quickKeys.presetKeys")

	data, err := json.Marshal(map[string]any{
		"deviceId":   mapValue(component, "deviceId"),
		"protocol":   mapValue(component, "protocol"),
		"keymap":     keymap,
		"presetKeys": presetKeys,
	})
	if err != nil {
		return QuickKeys{}, false, err
	}

	var keys QuickKeys
	if err := json.Unmarshal(data, &keys); err != nil {
		return QuickKeys{}, false, fmt.Errorf("quick keys: %w", err)
	}

	return keys, true, nil
}

func mapValue(value any, key string) any {
	object, _ := value.(map[string]any)
	return object[key]
}
