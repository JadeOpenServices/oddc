// Package fixture lays out what the tests run over, all from the
// repository's own catalog and evidence: deployments, machines that report
// a model's identity, and git checkouts of the catalog.
package fixture

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Repository is the root of this repository, found from the working
// directory up, as the tests run in their own folders below it.
var Repository = repository()

func repository() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			panic("no go.mod above the working directory")
		}
		dir = parent
	}
}

// Catalog loads the repository's catalog and its models, sorted.
func Catalog(t *testing.T) (*oddc.Registry, []string) {
	t.Helper()

	registry, err := oddc.LoadRegistry(Repository)
	if err != nil {
		t.Fatal(err)
	}

	var models []string
	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			models = append(models, id)
		}
	}
	sort.Strings(models)

	if len(models) == 0 {
		t.Fatal("catalog has no models")
	}

	return registry, models
}

// Write writes a file, making its directory.
func Write(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Sysfs lays out a machine that reports the identity a model declares;
// for model "" a machine that reports nothing.
func Sysfs(t *testing.T, registry *oddc.Registry, model string) string {
	t.Helper()

	sys := t.TempDir()
	if model == "" {
		return sys
	}

	for name, content := range Identity(t, registry, model).SysfsFiles() {
		Write(t, filepath.Join(sys, name), []byte(content))
	}

	return sys
}

// Identity is the identity a model declares.
func Identity(t *testing.T, registry *oddc.Registry, model string) oddc.MachineIdentity {
	t.Helper()

	identity, err := registry.ModelIdentity(model)
	if err != nil {
		t.Fatal(err)
	}

	return identity
}

// Components adds a model's components to the sysfs at sys, each with the
// driver its catalog entity names bound, as the kernel lays them out.
// Components the catalog names no driver for are added unbound.
func Components(t *testing.T, sys string, registry *oddc.Registry, model string) {
	t.Helper()

	resolved, err := registry.ResolveEntity(model)
	if err != nil {
		t.Fatal(err)
	}

	n := 0
	var walk func(object map[string]any)
	walk = func(object map[string]any) {
		bus, _ := object["bus"].(string)
		id, _ := object["deviceId"].(string)
		driver, _ := object["driver"].(string)
		if vendor, product, ok := strings.Cut(id, ":"); ok && bus != "" {
			n++
			var dir, bound string
			switch bus {
			case "pci":
				dir = filepath.Join(sys, "bus", "pci", "devices", fmt.Sprintf("0000:00:%02x.0", n))
				Write(t, filepath.Join(dir, "vendor"), []byte("0x"+vendor+"\n"))
				Write(t, filepath.Join(dir, "device"), []byte("0x"+product+"\n"))
				bound = dir
			case "usb":
				dir = filepath.Join(sys, "bus", "usb", "devices", fmt.Sprintf("1-%d", n))
				Write(t, filepath.Join(dir, "idVendor"), []byte(vendor+"\n"))
				Write(t, filepath.Join(dir, "idProduct"), []byte(product+"\n"))
				bound = dir + ":1.0"
			default: // hid and i2c
				bound = filepath.Join(sys, "bus", "hid", "devices",
					fmt.Sprintf("0018:%s:%s.%04X", strings.ToUpper(vendor), strings.ToUpper(product), n))
			}
			if err := os.MkdirAll(bound, 0o755); err != nil {
				t.Fatal(err)
			}
			if driver != "" {
				target := filepath.Join(sys, "bus", factsBus(bus), "drivers", driver)
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(bound, "driver")); err != nil {
					t.Fatal(err)
				}
			}
		}

		for _, key := range sortedKeys(object) {
			if child, ok := object[key].(map[string]any); ok {
				walk(child)
			}
		}
	}
	walk(resolved.Resolved)
}

func factsBus(bus string) string {
	if bus == "i2c" {
		return "hid"
	}
	return bus
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
