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

// Each built-in USB device carries its component's attachment, and is
// unsupported exactly when its entity marks it unusable on Linux.
func TestInternalUSBDevicesAttachmentAndSupport(t *testing.T) {
	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, id := range modelIDs(registry) {
		resolved, err := registry.ResolveEntity(id)
		if err != nil {
			t.Fatal(err)
		}
		devices, err := InternalUSBDevices(resolved.Resolved)
		if err != nil {
			t.Fatal(err)
		}
		unsupported := map[string]bool{}
		for _, part := range UnsupportedComponents(resolved.Resolved) {
			unsupported[part.Path] = true
		}
		for _, d := range devices {
			checked++
			attachment, _ := Lookup(resolved.Resolved, d.Path+".attachment")
			if d.Attachment != attachment {
				t.Errorf("%s %s: attachment %q, catalog %v", id, d.Path, d.Attachment, attachment)
			}
			if d.Supported == unsupported[d.Path] {
				t.Errorf("%s %s: supported %v, but unsupported in the catalog is %v", id, d.Path, d.Supported, unsupported[d.Path])
			}
		}
	}
	if checked == 0 {
		t.Fatal("no model has built-in USB devices")
	}
}
