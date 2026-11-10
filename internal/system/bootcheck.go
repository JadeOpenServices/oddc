// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunBootCheck checks that a trial generation brought up what it was built
// for: each --expect pci:VVVV:DDDD=DRIVER needs that driver bound to a PCI
// device with that ID (--sys, default /sys) and, for a GPU, a DRM card
// made by it. It fails otherwise, so a system that orders it before
// boot-complete.target falls back to the previous boot entry instead of
// keeping a black screen. It only reads sysfs.
func RunBootCheck(args []string) error {
	expected := cli.Values(args, "--expect")
	if len(expected) == 0 {
		return errors.New("name what to check with --expect pci:VVVV:DDDD=DRIVER")
	}
	sys := cli.Value(args, "--sys", "/sys")
	bound := oddc.ReadDrivers(sys)
	gpus := map[string][]oddc.GPU{}
	for _, gpu := range oddc.ReadGPUs(sys) {
		gpus[gpu.DeviceID] = append(gpus[gpu.DeviceID], gpu)
	}

	var failed []string
	for _, expect := range expected {
		device, driver, ok := strings.Cut(expect, "=")
		bus, id, _ := strings.Cut(device, ":")
		if !ok || bus != "pci" || driver == "" || !strings.Contains(id, ":") {
			return fmt.Errorf("%q is not pci:VVVV:DDDD=DRIVER", expect)
		}
		id = strings.ToLower(id)
		if !contains(bound["pci"][id], driver) {
			failed = append(failed, fmt.Sprintf("%s: %s is not bound (bound: %s)", device, driver, orNone(bound["pci"][id])))
			continue
		}
		for _, gpu := range gpus[id] {
			if gpu.Driver == driver && gpu.Mode != "native" {
				failed = append(failed, fmt.Sprintf("%s: %s made no DRM card", device, driver))
			}
		}
	}
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "; "))
	}
	fmt.Println("ok")
	return nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func orNone(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ", ")
}
