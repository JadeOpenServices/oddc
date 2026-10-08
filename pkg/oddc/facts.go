// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Facts are what a machine reports about itself: its DMI identity and the
// device IDs ("vvvv:pppp", lower case) it sees per bus. A bus is present
// only when it was read; an empty list means it was read and is empty.
type Facts struct {
	Identity MachineIdentity     `json:"identity"`
	Devices  map[string][]string `json:"devices,omitempty"`
}

// ModelFacts returns facts a machine of this model reports: its declared
// identity and the device IDs of its components.
func (r *Registry) ModelFacts(id string) (Facts, error) {
	identity, err := r.ModelIdentity(id)
	if err != nil {
		return Facts{}, err
	}

	resolved, err := r.ResolveEntity(id)
	if err != nil {
		return Facts{}, err
	}

	devices := map[string][]string{}
	for _, component := range modelComponents(resolved.Resolved) {
		bus := factsBus(component.Bus)
		devices[bus] = append(devices[bus], component.DeviceID)
	}

	return Facts{Identity: identity, Devices: devices}, nil
}

// ReadFacts reads a machine's facts from sysfs below sysRoot.
func ReadFacts(sysRoot string) Facts {
	return Facts{
		Identity: ReadIdentity(sysRoot),
		Devices:  ReadDevices(sysRoot),
	}
}

// ReadDevices lists the PCI, USB and HID device IDs below sysRoot. Buses
// without a sysfs directory are left out.
func ReadDevices(sysRoot string) map[string][]string {
	devices := map[string][]string{}

	read := func(dir, name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return ""
		}

		return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(string(data)), "0x"))
	}

	scan := func(bus string, id func(dir string) string) {
		entries, err := os.ReadDir(filepath.Join(sysRoot, "bus", bus, "devices"))
		if err != nil {
			return
		}

		seen := map[string]bool{}
		list := []string{}

		for _, entry := range entries {
			deviceID := id(filepath.Join(sysRoot, "bus", bus, "devices", entry.Name()))
			if deviceID != "" && !seen[deviceID] {
				seen[deviceID] = true
				list = append(list, deviceID)
			}
		}

		sort.Strings(list)
		devices[bus] = list
	}

	scan("pci", func(dir string) string {
		vendor, device := read(dir, "vendor"), read(dir, "device")
		if vendor == "" || device == "" {
			return ""
		}
		return vendor + ":" + device
	})

	// USB interfaces have no idVendor and are skipped.
	scan("usb", func(dir string) string {
		vendor, product := read(dir, "idVendor"), read(dir, "idProduct")
		if vendor == "" || product == "" {
			return ""
		}
		return vendor + ":" + product
	})

	// HID devices are named BUS:VENDOR:PRODUCT.INSTANCE.
	scan("hid", func(dir string) string {
		name, _, _ := strings.Cut(filepath.Base(dir), ".")
		parts := strings.Split(name, ":")
		if len(parts) != 3 {
			return ""
		}
		return strings.ToLower(parts[1] + ":" + parts[2])
	})

	return devices
}
