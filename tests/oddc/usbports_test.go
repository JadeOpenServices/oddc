// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// Every usbPort in the catalog names a USB controller placed in the same
// model and a valid hub, port and connect type; no two internal devices
// share a port; and a broken usbPort is refused.
func TestInternalUSBDevices(t *testing.T) {
	registry, models := fixture.Catalog(t)
	found := 0

	for _, model := range models {
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		devices, err := InternalUSBDevices(resolved.Resolved)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		found += len(devices)

		taken := map[string]string{}
		for _, device := range devices {
			key := fmt.Sprintf("%s %s %d", device.Controller, device.Hub, device.Port)
			if other, ok := taken[key]; ok {
				t.Errorf("%s: %s and %s share %s", model, other, device.Path, key)
			}
			taken[key] = device.Path
		}
		if len(devices) == 0 {
			continue
		}

		// A copy with one port on a hub that doesn't exist must be refused.
		data, _ := json.Marshal(resolved.Resolved)
		var broken map[string]any
		_ = json.Unmarshal(data, &broken)
		object := broken
		for _, key := range splitPath(devices[0].Path) {
			object = object[key].(map[string]any)
		}
		object["usbPort"].(map[string]any)["hub"] = "usb9"
		if _, err := InternalUSBDevices(broken); err == nil {
			t.Errorf("%s: usbPort with hub usb9 accepted", model)
		}
	}

	if found == 0 {
		t.Fatal("no model places a USB device on a known port")
	}
}

func splitPath(path string) []string {
	var keys []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			keys = append(keys, path[start:i])
			start = i + 1
		}
	}
	return keys
}
