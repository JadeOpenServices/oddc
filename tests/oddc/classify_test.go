// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// The facts a model declares classify to that model, with every
// component it declares present.
func TestCatalogModelFactsClassifyToModel(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		facts, err := registry.ModelFacts(id)
		if err != nil {
			t.Fatal(err)
		}

		got, err := registry.Classify(facts)
		if err != nil {
			t.Fatal(err)
		}

		if got.Result != ResultMatched || got.Model != id {
			t.Errorf("%s: classified as %s %q %v", id, got.Result, got.Model, got.Ambiguous)
			continue
		}

		for _, candidate := range got.Candidates {
			if candidate.Model != id {
				continue
			}

			for _, component := range candidate.Components {
				if component.Present == nil || !*component.Present {
					t.Errorf("%s: component %s not present", id, component.Path)
				}
			}
		}
	}
}

func TestCatalogEmptyFactsClassifyToNone(t *testing.T) {
	got, err := catalog(t).Classify(Facts{})
	if err != nil {
		t.Fatal(err)
	}

	if got.Result != ResultNone || len(got.Candidates) != 0 {
		t.Fatalf("got %+v", got)
	}
}

// writeSysfsDevices lays out device IDs per bus as the kernel shows them.
func writeSysfsDevices(t *testing.T, sys string, devices map[string][]string) {
	t.Helper()

	files := map[string]string{}

	for bus, ids := range devices {
		for i, id := range ids {
			vendor, product, _ := strings.Cut(id, ":")

			switch bus {
			case "pci":
				dir := filepath.Join("bus", "pci", "devices", fmt.Sprintf("0000:00:%02x.0", i))
				files[filepath.Join(dir, "vendor")] = "0x" + vendor + "\n"
				files[filepath.Join(dir, "device")] = "0x" + product + "\n"

			case "usb":
				dir := filepath.Join("bus", "usb", "devices", fmt.Sprintf("1-%d", i+1))
				files[filepath.Join(dir, "idVendor")] = vendor + "\n"
				files[filepath.Join(dir, "idProduct")] = product + "\n"

			case "hid":
				name := fmt.Sprintf(
					"0018:%s:%s.%04X",
					strings.ToUpper(vendor),
					strings.ToUpper(product),
					i+1,
				)
				files[filepath.Join("bus", "hid", "devices", name, "uevent")] = ""

			default:
				t.Fatalf("no sysfs layout for bus %q", bus)
			}
		}
	}

	for name, content := range files {
		path := filepath.Join(sys, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A machine reporting a model's facts through sysfs is read back with
// those facts and classified as that model.
func TestCatalogReadFactsFromSysfs(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		facts, err := registry.ModelFacts(id)
		if err != nil {
			t.Fatal(err)
		}

		sys := t.TempDir()
		for name, content := range facts.Identity.SysfsFiles() {
			path := filepath.Join(sys, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		writeSysfsDevices(t, sys, facts.Devices)

		read := ReadFacts(sys)

		for bus, want := range facts.Devices {
			want = slices.Compact(slices.Sorted(slices.Values(want)))
			if got := read.Devices[bus]; !slices.Equal(got, want) {
				t.Errorf("%s: %s read %v want %v", id, bus, got, want)
			}
		}

		got, err := registry.Classify(read)
		if err != nil {
			t.Fatal(err)
		}

		if got.Result != ResultMatched || got.Model != id {
			t.Errorf("%s: classified as %s %q", id, got.Result, got.Model)
		}
	}
}

// A bus without a sysfs directory is left out, so its components are
// reported as unknown, not as missing.
func TestReadDevicesLeavesOutUnreadBuses(t *testing.T) {
	sys := t.TempDir()
	writeSysfsDevices(t, sys, map[string][]string{"usb": {"1d6b:0002"}})

	got := ReadDevices(sys)

	buses := make([]string, 0, len(got))
	for bus := range got {
		buses = append(buses, bus)
	}
	sort.Strings(buses)

	if !slices.Equal(buses, []string{"usb"}) {
		t.Fatalf("buses %v", buses)
	}
}
