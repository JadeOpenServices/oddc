// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"os"
	"path/filepath"
	"slices"
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

	for bus, id := range busIDs {
		entries, err := os.ReadDir(filepath.Join(sysRoot, "bus", bus, "devices"))
		if err != nil {
			continue
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

	return devices
}

// ReadDrivers lists the kernel drivers bound to each device ID below
// sysRoot, per bus. USB drivers bind to a device's interfaces, such as
// 1-4:1.0, and are listed under the ID of the device, 1-4.
func ReadDrivers(sysRoot string) map[string]map[string][]string {
	drivers := map[string]map[string][]string{}

	for bus, id := range busIDs {
		dir := filepath.Join(sysRoot, "bus", bus, "devices")
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			link, err := os.Readlink(filepath.Join(dir, entry.Name(), "driver"))
			if err != nil {
				continue
			}

			device := entry.Name()
			if bus == "usb" {
				parent, _, isInterface := strings.Cut(device, ":")
				if !isInterface {
					continue
				}
				device = parent
			}

			deviceID := id(filepath.Join(dir, device))
			if deviceID == "" {
				continue
			}
			if drivers[bus] == nil {
				drivers[bus] = map[string][]string{}
			}
			if driver := filepath.Base(link); !slices.Contains(drivers[bus][deviceID], driver) {
				drivers[bus][deviceID] = append(drivers[bus][deviceID], driver)
				sort.Strings(drivers[bus][deviceID])
			}
		}
	}

	return drivers
}

// readID reads a sysfs ID file as lower-case hex without 0x.
func readID(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}

	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(string(data)), "0x"))
}

// busIDs reads the device ID ("vvvv:pppp") of a sysfs device directory,
// per bus, or "".
var busIDs = map[string]func(dir string) string{
	"pci": func(dir string) string {
		vendor, device := readID(dir, "vendor"), readID(dir, "device")
		if vendor == "" || device == "" {
			return ""
		}
		return vendor + ":" + device
	},

	// USB interfaces have no idVendor and are skipped.
	"usb": func(dir string) string {
		vendor, product := readID(dir, "idVendor"), readID(dir, "idProduct")
		if vendor == "" || product == "" {
			return ""
		}
		return vendor + ":" + product
	},

	// HID devices are named BUS:VENDOR:PRODUCT.INSTANCE.
	"hid": func(dir string) string {
		name, _, _ := strings.Cut(filepath.Base(dir), ".")
		parts := strings.Split(name, ":")
		if len(parts) != 3 {
			return ""
		}
		return strings.ToLower(parts[1] + ":" + parts[2])
	},
}
