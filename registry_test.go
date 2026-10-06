package oddc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func repositoryODDCRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(
		".",
	)
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func TestRepositoryRegistryLoads(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	if len(registry.Entities) == 0 {
		t.Fatal("registry contains no entities")
	}
}

func TestFrameworkNormalizedGraphResolves(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := registry.ResolveEntity(
		"model/framework/laptop-13-amd-ryzen-7040",
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]any{
		"vendor.name":                                                 "Framework",
		"family.model.family":                                         "Laptop 13",
		"hardware.graphics.integrated.deviceId":                       "1002:15bf",
		"hardware.graphics.integrated.driver":                         "amdgpu",
		"hardware.network.wifi.primary.deviceId":                      "10ec:b852",
		"hardware.network.wifi.primary.driver":                        "rtw89_8852be",
		"hardware.network.bluetooth.primary.deviceId":                 "13d3:3571",
		"hardware.security.fingerprint.primary.deviceId":              "27c6:609c",
		"hardware.input.touchpad.primary.deviceId":                    "093a:0274",
		"hardware.input.touchpad.primary.bus":                         "i2c",
		"hardware.input.touchpad.primary.attachment":                  "internal",
		"hardware.network.bluetooth.primary.attachment":               "internal",
		"hardware.security.fingerprint.primary.attachment":            "internal",
		"hardware.input.touchpad.primary.driver":                      "hid-multitouch",
		"policy.thermal.fanControl.policy.thermalEnterC":              82,
		"policy.thermal.fanControl.policy.thermalExitC":               74,
		"class.policy.power.batteryProtection.lowWarningPercent":      10,
		"class.policy.power.batteryProtection.criticalWarningPercent": 5,
		"class.policy.power.batteryProtection.shutdownPercent":        2,
		"class.policy.power.chargeThresholds.startPercent":            75,
		"class.policy.power.chargeThresholds.endPercent":              95,
		"quirks.fprintdResume.fprintd.noTimeout":                      true,
		"quirks.fprintdResume.fprintd.restartAfterSleep":              true,
		"quirks.usbCExpansionPower.runtimePower.control":              "on",
		"vendor.policy.secureBoot.setupModeStrategy":                  "clear-platform-key",
		"quirks.linux72DcnFreeze.fallbackPackage":                     "linuxPackages_7_1",
	}

	for path, want := range tests {
		got, exists := Lookup(resolved.Resolved, path)
		if !exists {
			t.Fatalf("%s missing", path)
		}

		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("%s=%v want=%v", path, got, want)
		}
	}
}

func TestRegistryRejectsMissingReference(t *testing.T) {
	root := t.TempDir()

	writeEntityFixture(
		t,
		root,
		"catalog/entities/models/bad.json",
		`{
  "apiVersion": "oddc.openjade.de/v2",
  "kind": "DeviceModel",
  "metadata": {
    "id": "model/test/bad",
    "name": "Bad"
  },
  "data": {
    "graphics": {
      "ref": "graphics/missing/device"
    }
  }
}`,
	)

	if _, err := LoadRegistry(root); err == nil {
		t.Fatal("registry accepted missing reference")
	}
}

func TestRegistryRejectsReferenceCycle(t *testing.T) {
	root := t.TempDir()

	writeEntityFixture(
		t,
		root,
		"catalog/entities/a.json",
		`{
  "apiVersion": "oddc.openjade.de/v2",
  "kind": "Test",
  "metadata": {
    "id": "test/a",
    "name": "A"
  },
  "data": {
    "next": {
      "ref": "test/b"
    }
  }
}`,
	)

	writeEntityFixture(
		t,
		root,
		"catalog/entities/b.json",
		`{
  "apiVersion": "oddc.openjade.de/v2",
  "kind": "Test",
  "metadata": {
    "id": "test/b",
    "name": "B"
  },
  "data": {
    "next": {
      "ref": "test/a"
    }
  }
}`,
	)

	if _, err := LoadRegistry(root); err == nil {
		t.Fatal("registry accepted reference cycle")
	}
}

func writeEntityFixture(
	t *testing.T,
	root string,
	relative string,
	data string,
) {
	t.Helper()

	path := filepath.Join(root, relative)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}
