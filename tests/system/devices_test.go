// SPDX-License-Identifier: GPL-3.0-or-later

package system_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// catalogRoot is the catalog and its evidence with no model deployed, as
// the installer has them on the live system.
func catalogRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"catalog", "evidence"} {
		if err := os.Symlink(filepath.Join(fixture.Repository, dir), filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// addGPU lays out a PCI display controller at slot with the driver bound,
// a DRM card when card is set, and the kernel's removable mark when set.
func addGPU(t *testing.T, sys, slot, deviceID, driver string, card, removable bool) {
	t.Helper()
	vendor, product, _ := strings.Cut(deviceID, ":")
	dir := filepath.Join(sys, "bus", "pci", "devices", slot)
	fixture.Write(t, filepath.Join(dir, "vendor"), []byte("0x"+vendor+"\n"))
	fixture.Write(t, filepath.Join(dir, "device"), []byte("0x"+product+"\n"))
	fixture.Write(t, filepath.Join(dir, "class"), []byte("0x030000\n"))
	if removable {
		fixture.Write(t, filepath.Join(dir, "removable"), []byte("removable\n"))
	}
	if driver != "" {
		target := filepath.Join(sys, "bus", "pci", "drivers", driver)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "driver")); err != nil {
			t.Fatal(err)
		}
	}
	if card {
		if err := os.MkdirAll(filepath.Join(dir, "drm", "card0"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

type bestEffortOut struct {
	Model      *string            `json:"model"`
	Devices    []oddc.PresentPart `json:"devices"`
	Graphics   []oddc.GPU         `json:"graphics"`
	Contribute oddc.Contribute    `json:"contribute"`
	LiveBoot   oddc.LiveBoot      `json:"liveBoot"`
}

func runDevices(t *testing.T, args ...string) bestEffortOut {
	t.Helper()
	out := fixture.Stdout(t, func() error { return system.RunDevices(append([]string{"devices", "--json"}, args...)) })
	var got bestEffortOut
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return got
}

// proven is a GPU a verified model sets a driver branch for, with the
// driver its evidence bound.
type proven struct{ model, component, deviceID, branch, driver string }

func provenGPUs(t *testing.T, registry *oddc.Registry, models []string) []proven {
	t.Helper()
	var found []proven
	for _, model := range models {
		status, err := registry.Verify(model)
		if err != nil {
			t.Fatal(err)
		}
		if status.Status != oddc.Verified {
			continue
		}
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		graphics, _ := oddc.Lookup(resolved.Resolved, "hardware.graphics")
		roles, _ := graphics.(map[string]any)
		for role, value := range roles {
			gpu, _ := value.(map[string]any)
			branch, _ := oddc.Lookup(resolved.Resolved, "policy.graphics."+role+".driverBranch")
			id, _ := gpu["id"].(string)
			deviceID, _ := gpu["deviceId"].(string)
			text, _ := branch.(string)
			if text == "" || gpu["bus"] != "pci" || len(status.Evidence.Drivers[id]) == 0 {
				continue
			}
			found = append(found, proven{model, id, deviceID, text, status.Evidence.Drivers[id][0]})
		}
	}
	return found
}

// A machine no model matches gets model null, its parts with their
// catalog components, and for a GPU a verified model set a driver branch
// for, that branch as an option tested on that model. A GPU behind an
// external port gets none, and nobody's chip gets an option for another.
func TestDevicesBestEffortGraphicsOptions(t *testing.T) {
	registry, models := fixture.Catalog(t)
	root := catalogRoot(t)
	gpus := provenGPUs(t, registry, models)
	if len(gpus) == 0 {
		t.Fatal("no verified model sets a graphics driver branch")
	}

	for _, gpu := range gpus {
		sys := fixture.Sysfs(t, registry, "")
		addGPU(t, sys, "0000:01:00.0", gpu.deviceID, "", false, false)
		addGPU(t, sys, "0000:02:00.0", gpu.deviceID, "", false, true)

		got := runDevices(t, "--root", root, "--sys", sys)
		if got.Model != nil {
			t.Fatalf("%s: model %q on an unknown machine", gpu.model, *got.Model)
		}
		if got.Contribute.ReportEndpoint != nil && !strings.HasPrefix(*got.Contribute.ReportEndpoint, "https://") {
			t.Errorf("report endpoint %q", *got.Contribute.ReportEndpoint)
		}
		if len(got.Graphics) != 2 {
			t.Fatalf("%s: graphics %+v", gpu.model, got.Graphics)
		}
		fixed, external := got.Graphics[0], got.Graphics[1]
		if fixed.ID != gpu.component || fixed.Mode != "basic" || external.Removable != true || len(external.Options) != 0 {
			t.Errorf("%s: graphics %+v", gpu.model, got.Graphics)
		}
		found := false
		for _, option := range fixed.Options {
			if option.ProvenOn.Model == gpu.model && option.DriverBranch == gpu.branch && option.Drivers[0] == gpu.driver && option.ProvenOn.Evidence != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: options %+v, want %s", gpu.model, fixed.Options, gpu.branch)
		}
		for _, part := range got.Devices {
			if part.Proven != "untested" {
				t.Errorf("%s: %s proven %q on an unknown machine", gpu.model, part.DeviceID, part.Proven)
			}
		}
	}

	// Every other GPU of the catalog gets no option from these.
	for _, model := range models {
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		graphics, _ := oddc.Lookup(resolved.Resolved, "hardware.graphics")
		roles, _ := graphics.(map[string]any)
		for role, value := range roles {
			gpu, _ := value.(map[string]any)
			deviceID, _ := gpu["deviceId"].(string)
			driver, _ := gpu["driver"].(string)
			if gpu["bus"] != "pci" || deviceID == "" || driver == "" || isProven(gpus, deviceID) {
				continue
			}
			sys := fixture.Sysfs(t, registry, "")
			addGPU(t, sys, "0000:03:00.0", deviceID, driver, true, false)
			got := runDevices(t, "--root", root, "--sys", sys)
			if len(got.Graphics) != 1 || got.Graphics[0].Mode != "native" || len(got.Graphics[0].Options) != 0 {
				t.Errorf("%s %s: graphics %+v", model, role, got.Graphics)
			}
		}
	}
}

func isProven(gpus []proven, deviceID string) bool {
	for _, gpu := range gpus {
		if gpu.deviceID == deviceID {
			return true
		}
	}
	return false
}

// A machine a model matches is that model, with nothing deployed yet.
func TestDevicesClassifiesTheLiveMachine(t *testing.T) {
	registry, models := fixture.Catalog(t)
	root := catalogRoot(t)
	for _, model := range models {
		sys := fixture.Sysfs(t, registry, model)
		fixture.Components(t, sys, registry, model)
		got := runDevices(t, "--root", root, "--sys", sys)
		if got.Model == nil || *got.Model != model {
			t.Errorf("%s: model %v", model, got.Model)
		}
	}
}

// The boot check passes only when each expected driver is bound to its
// device and, for a GPU, made a DRM card.
func TestBootCheck(t *testing.T) {
	registry, models := fixture.Catalog(t)
	for _, gpu := range provenGPUs(t, registry, models) {
		expect := fmt.Sprintf("pci:%s=%s", gpu.deviceID, gpu.driver)
		for _, c := range []struct {
			name       string
			driver     string
			card, pass bool
		}{
			{"bound with a card", gpu.driver, true, true},
			{"bound without a card", gpu.driver, false, false},
			{"other driver", "simpledrm", true, false},
			{"no driver", "", false, false},
		} {
			sys := t.TempDir()
			addGPU(t, sys, "0000:01:00.0", gpu.deviceID, c.driver, c.card, false)
			err := system.RunBootCheck([]string{"bootcheck", "--sys", sys, "--expect", expect})
			if (err == nil) != c.pass {
				t.Errorf("%s %s: %v", gpu.model, c.name, err)
			}
		}
	}
	if system.RunBootCheck([]string{"bootcheck"}) == nil {
		t.Error("no --expect passed")
	}
}

// The installed base boots the way the live system did: the
// graphics-safety parameters of its command line, and only those, are
// reported.
func TestDevicesLiveBootParameters(t *testing.T) {
	registry, _ := fixture.Catalog(t)
	root := catalogRoot(t)
	for cmdline, want := range map[string]string{
		"BOOT_IMAGE=/boot/bzImage init=/nix/store/x-init quiet":              "",
		"BOOT_IMAGE=/boot/bzImage nomodeset quiet splash":                    "nomodeset",
		"nouveau.modeset=0 i915.modeset=1 amdgpu.modeset=0 root=LABEL=nixos": "nouveau.modeset=0 amdgpu.modeset=0",
		"nomodeset=1 ../x.modeset=0 loglevel=4":                              "",
	} {
		sys := fixture.Sysfs(t, registry, "")
		proc := t.TempDir()
		fixture.Write(t, filepath.Join(proc, "cmdline"), []byte(cmdline+"\n"))
		got := runDevices(t, "--root", root, "--sys", sys, "--proc", proc)
		if strings.Join(got.LiveBoot.Parameters, " ") != want {
			t.Errorf("%q: %v, want %q", cmdline, got.LiveBoot.Parameters, want)
		}
	}
}
