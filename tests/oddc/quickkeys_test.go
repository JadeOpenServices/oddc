// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// quickKeysOf resolves a model and reads its Quick Keys.
func quickKeysOf(t *testing.T, registry *Registry, model string) (QuickKeys, bool) {
	t.Helper()

	resolved, err := registry.ResolveModel(model, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	quickKeys, ok, err := QuickKeysFrom(resolved.Resolved)
	if err != nil {
		t.Fatal(err)
	}

	return quickKeys, ok
}

// readCapture reads the hidraw reports recorded on a real machine of the
// model, one hex report per line.
func readCapture(t *testing.T, model string) [][]byte {
	t.Helper()

	path := filepath.Join("tests", "oddc", "testdata", "quickkeys", strings.TrimPrefix(model, "model/")+".hex")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s has mappable Quick Keys but no capture: %v", model, err)
	}

	var reports [][]byte
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		report, err := hex.DecodeString(strings.ReplaceAll(line, " ", ""))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		reports = append(reports, report)
	}

	return reports
}

// Replaying a real capture through the decoder sends, for each button
// press, the key the keymap names for the preset the report shows; the
// preset switch sends nothing; every key is let go; and the capture
// reaches every key of the keymap.
func TestQuickKeysCapture(t *testing.T) {
	registry, models := fixture.Catalog(t)
	tested := 0

	for _, model := range models {
		quickKeys, ok := quickKeysOf(t, registry, model)
		if !ok {
			continue
		}
		tested++

		decoder, err := NewQuickKeysDecoder(quickKeys)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}

		buttonName := map[int]string{}
		for name, button := range quickKeys.Protocol.Buttons {
			buttonName[button.Raw] = name
		}
		presetName := map[int]string{}
		for name, preset := range quickKeys.Protocol.Presets {
			presetName[preset.Raw] = name
		}

		protocol := quickKeys.Protocol
		pressed := map[uint16]int{}
		held := map[uint16]bool{}
		previous := 0

		for i, report := range readCapture(t, model) {
			events := decoder.Decode(report)
			buttons := int(report[protocol.ButtonByteOffset])
			newly := buttons &^ previous
			previous = buttons

			var want []string
			for bit := 1; bit < 0x100; bit <<= 1 {
				name, known := buttonName[bit]
				if newly&bit == 0 || !known || protocol.Buttons[name].PresetSwitch {
					continue
				}
				want = append(want, quickKeys.Keymap[presetName[int(report[protocol.PresetByteOffset])]][name])
			}

			var got []string
			for _, event := range events {
				if event.Pressed {
					if held[event.Code] {
						t.Fatalf("%s report %d: key %d pressed twice", model, i, event.Code)
					}
					held[event.Code] = true
					pressed[event.Code]++
					got = append(got, keyName(event.Code))
				} else {
					if !held[event.Code] {
						t.Fatalf("%s report %d: key %d released while up", model, i, event.Code)
					}
					delete(held, event.Code)
				}
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("%s report %d (% x): pressed %v, want %v", model, i, report, got, want)
			}
		}

		if len(held) != 0 {
			t.Fatalf("%s: keys still held after the capture: %v", model, held)
		}
		for preset, buttons := range quickKeys.Keymap {
			for button, key := range buttons {
				if pressed[KeyCodes[key]] == 0 {
					t.Errorf("%s: capture never reaches %s.%s (%s)", model, preset, button, key)
				}
			}
		}
	}

	if tested == 0 {
		t.Fatal("no model has mappable Quick Keys")
	}
}

func keyName(code uint16) string {
	for name, value := range KeyCodes {
		if value == code {
			return name
		}
	}
	return ""
}

// A keymap is refused when it names a key, button or preset that does not
// exist, or gives the preset switch a key.
func TestQuickKeysKeymapRefused(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		quickKeys, ok := quickKeysOf(t, registry, model)
		if !ok {
			continue
		}

		var preset, button, presetSwitch string
		for name := range quickKeys.Keymap {
			preset = name
		}
		for name := range quickKeys.Keymap[preset] {
			button = name
		}
		for name, value := range quickKeys.Protocol.Buttons {
			if value.PresetSwitch {
				presetSwitch = name
			}
		}

		for _, change := range []struct {
			name   string
			preset string
			button string
			key    string
		}{
			{"unknown key", preset, button, "KEY_NOT_A_KEY"},
			{"unknown button", preset, "button99", "KEY_MACRO1"},
			{"unknown preset", "preset99", button, "KEY_MACRO1"},
			{"preset switch", preset, presetSwitch, "KEY_MACRO1"},
		} {
			if change.button == "" {
				continue
			}
			broken := quickKeys
			broken.Keymap = map[string]map[string]string{change.preset: {change.button: change.key}}

			if _, err := NewQuickKeysDecoder(broken); err == nil {
				t.Errorf("%s: keymap with %s accepted", model, change.name)
			}
		}
	}
}
