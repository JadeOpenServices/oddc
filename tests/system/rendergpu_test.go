// SPDX-License-Identifier: GPL-3.0-or-later

package system_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// drmSysfs builds /sys/class/drm with one card per GPU of the model, its
// device bound to the GPU's driver, and the given connectors per card.
func drmSysfs(t *testing.T, cards map[string]string, connectors map[string]string) string {
	t.Helper()

	sys := t.TempDir()
	for card, driver := range cards {
		device := filepath.Join(sys, "devices", card)
		if err := os.MkdirAll(filepath.Join(sys, "bus", "pci", "drivers", driver), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(device, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(sys, "bus", "pci", "drivers", driver), filepath.Join(device, "driver")); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(sys, "class", "drm", card), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(device, filepath.Join(sys, "class", "drm", card, "device")); err != nil {
			t.Fatal(err)
		}
	}
	for connector, status := range connectors {
		fixture.Write(t, filepath.Join(sys, "class", "drm", connector, "status"), []byte(status+"\n"))
	}
	return sys
}

// On every model that enables the render quirk, the compositor renders on
// the default GPU while only its screens are connected and on the other GPU
// once one of that GPU's screens is.
func TestRenderGPUFollowsConnectedScreens(t *testing.T) {
	registry, models := fixture.Catalog(t)

	found := 0
	for _, model := range models {
		resolved, err := registry.ResolveModel(model, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		rule, ok, err := oddc.RenderRuleFrom(resolved.Resolved)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		if !ok {
			continue
		}
		found++

		drivers := oddc.GraphicsDrivers(resolved.Resolved)
		other := ""
		for gpu := range drivers {
			if gpu != rule.Default {
				other = gpu
			}
		}
		if other == "" {
			t.Fatalf("%s: the render quirk needs a second GPU", model)
		}
		cards := map[string]string{"card0": drivers[other], "card1": drivers[rule.Default]}
		root := fixture.Deployment(t, registry, model, false)
		run := func(connectors map[string]string) string {
			sys := drmSysfs(t, cards, connectors)
			return strings.TrimSpace(fixture.Stdout(t, func() error {
				return system.RunRenderGPU([]string{"oddc", "rendergpu", "--resolved", filepath.Join(root, "resolved.json"), "--sys", sys})
			}))
		}

		if got := run(map[string]string{"card1-eDP-1": "connected", "card0-DP-3": "disconnected"}); got != "/dev/dri/card1:/dev/dri/card0" {
			t.Errorf("%s, only the default GPU's screen: %q, want the default GPU first", model, got)
		}
		if got := run(map[string]string{"card1-eDP-1": "connected", "card0-DP-3": "connected"}); got != "/dev/dri/card0:/dev/dri/card1" {
			t.Errorf("%s, a screen on %s connected: %q, want %s first", model, other, got, other)
		}
	}

	if found == 0 {
		t.Fatal("no model enables a render quirk")
	}
}

// A model without the quirk gets an error, never a guessed GPU.
func TestRenderGPURefusesWithoutQuirk(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		resolved, err := registry.ResolveModel(model, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := oddc.RenderRuleFrom(resolved.Resolved); ok {
			continue
		}
		root := fixture.Deployment(t, registry, model, false)
		err = system.RunRenderGPU([]string{"oddc", "rendergpu", "--resolved", filepath.Join(root, "resolved.json"), "--sys", t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), "no renderGpu quirk") {
			t.Errorf("%s: want a refusal without the quirk, got %v", model, err)
		}
		return
	}
	t.Skip("every model enables a render quirk")
}
