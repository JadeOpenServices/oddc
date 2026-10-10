// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"encoding/json"
	"fmt"
	"sort"
)

// QuickKeys is what a resolved model says about its Quick Keys: the
// component at hardware.input.quickKeys.primary and the key each button
// sends per preset, from policy.input.quickKeys.keymap.
type QuickKeys struct {
	DeviceID string                       `json:"deviceId"`
	Protocol QuickKeysProtocol            `json:"protocol"`
	Keymap   map[string]map[string]string `json:"keymap"`
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

	data, err := json.Marshal(map[string]any{
		"deviceId": mapValue(component, "deviceId"),
		"protocol": mapValue(component, "protocol"),
		"keymap":   keymap,
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

// KeyEvent is a key going down or up.
type KeyEvent struct {
	Code    uint16
	Pressed bool
}

// QuickKeysDecoder turns input reports into key events. A key is chosen
// when its button goes down, from the preset the same report names, and
// released with that same key, so a preset change never leaves a key held.
type QuickKeysDecoder struct {
	protocol QuickKeysProtocol
	keys     map[int]map[int]uint16 // preset raw -> button raw -> key
	held     map[int]uint16         // button raw -> key sent at press
}

// NewQuickKeysDecoder checks the keymap against the protocol and the key
// names this program knows. Every preset and button in the keymap must
// exist; the preset switch sends no key.
func NewQuickKeysDecoder(quickKeys QuickKeys) (*QuickKeysDecoder, error) {
	protocol := quickKeys.Protocol
	if protocol.ReportLength < 1 ||
		protocol.ButtonByteOffset < 1 || protocol.ButtonByteOffset >= protocol.ReportLength ||
		protocol.PresetByteOffset < 1 || protocol.PresetByteOffset >= protocol.ReportLength {
		return nil, fmt.Errorf("quick keys: protocol offsets outside a %d byte report", protocol.ReportLength)
	}

	decoder := &QuickKeysDecoder{
		protocol: protocol,
		keys:     map[int]map[int]uint16{},
		held:     map[int]uint16{},
	}

	for presetName, buttons := range quickKeys.Keymap {
		preset, ok := protocol.Presets[presetName]
		if !ok {
			return nil, fmt.Errorf("quick keys: keymap names unknown preset %q", presetName)
		}
		decoder.keys[preset.Raw] = map[int]uint16{}

		for buttonName, keyName := range buttons {
			button, ok := protocol.Buttons[buttonName]
			switch {
			case !ok:
				return nil, fmt.Errorf("quick keys: keymap names unknown button %q", buttonName)
			case button.PresetSwitch:
				return nil, fmt.Errorf("quick keys: %s switches presets and sends no key", buttonName)
			}

			code, ok := KeyCodes[keyName]
			if !ok {
				return nil, fmt.Errorf("quick keys: %s.%s: unknown key %q", presetName, buttonName, keyName)
			}
			decoder.keys[preset.Raw][button.Raw] = code
		}
	}

	return decoder, nil
}

// Codes lists every key the keymap can send, sorted.
func (d *QuickKeysDecoder) Codes() []uint16 {
	seen := map[uint16]bool{}
	var codes []uint16
	for _, buttons := range d.keys {
		for _, code := range buttons {
			if !seen[code] {
				seen[code] = true
				codes = append(codes, code)
			}
		}
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })

	return codes
}

// Decode returns the key events one input report causes. Reports of
// another ID, length or framing are ignored.
func (d *QuickKeysDecoder) Decode(report []byte) []KeyEvent {
	protocol := d.protocol
	if len(report) != protocol.ReportLength || int(report[0]) != protocol.ReportID {
		return nil
	}
	for name, want := range protocol.Framing {
		var offset int
		if _, err := fmt.Sscanf(name, "byte%d", &offset); err != nil ||
			offset >= len(report) || int(report[offset]) != want {
			return nil
		}
	}

	pressed := int(report[protocol.ButtonByteOffset])
	preset := int(report[protocol.PresetByteOffset])
	var events []KeyEvent

	for bit := 0; bit < 8; bit++ {
		raw := 1 << bit
		code, held := d.held[raw]

		switch down := pressed&raw != 0; {
		case down && !held:
			if code, ok := d.keys[preset][raw]; ok {
				d.held[raw] = code
				events = append(events, KeyEvent{Code: code, Pressed: true})
			}
		case !down && held:
			delete(d.held, raw)
			events = append(events, KeyEvent{Code: code, Pressed: false})
		}
	}

	return events
}

// Release returns the events that let go of every key still held.
func (d *QuickKeysDecoder) Release() []KeyEvent {
	var events []KeyEvent
	for raw, code := range d.held {
		delete(d.held, raw)
		events = append(events, KeyEvent{Code: code, Pressed: false})
	}

	return events
}
