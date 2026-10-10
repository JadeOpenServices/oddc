// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunQuickKeys turns a Quick Keys device's hidraw reports into key
// presses on a uinput keyboard, following the resolved model's protocol
// and keymap (--resolved, default /etc/oddc/resolved.json). It reads only
// --device, and only when that hidraw node is the model's device. With
// --state DIR it writes the active preset's name to DIR/preset whenever a
// report shows it changed.
func RunQuickKeys(args []string) error {
	data, err := os.ReadFile(cli.Value(args, "--resolved", "/etc/oddc/resolved.json"))
	if err != nil {
		return err
	}
	var resolved map[string]any
	if err := json.Unmarshal(data, &resolved); err != nil {
		return err
	}

	quickKeys, ok, err := oddc.QuickKeysFrom(resolved)
	switch {
	case err != nil:
		return err
	case !ok:
		return errors.New("the resolved model has no mappable Quick Keys")
	}
	decoder, err := oddc.NewQuickKeysDecoder(quickKeys)
	if err != nil {
		return err
	}
	if len(decoder.Codes()) == 0 {
		return errors.New("the resolved model maps no Quick Keys (policy.input.quickKeys.keymap)")
	}

	device, err := os.Open(cli.Value(args, "--device", "/dev/oddc/quickkeys"))
	if err != nil {
		return err
	}
	defer device.Close()

	if err := checkHidraw(device, quickKeys.DeviceID); err != nil {
		return err
	}

	vendor, product := splitDeviceID(quickKeys.DeviceID)
	keys, err := newKeyboard(cli.Value(args, "--uinput", "/dev/uinput"), "ODDC Quick Keys", vendor, product, decoder.Codes())
	if err != nil {
		return err
	}
	defer keys.Close()
	defer keys.Send(decoder.Release())

	stateDir := cli.Value(args, "--state", "")
	preset := ""
	report := make([]byte, 64)
	for {
		n, err := device.Read(report)
		if err != nil {
			return fmt.Errorf("reading %s: %w", device.Name(), err)
		}
		if now := decoder.Preset(report[:n]); stateDir != "" && now != "" && now != preset {
			if err := writePreset(stateDir, now); err != nil {
				return err
			}
			preset = now
		}
		if err := keys.Send(decoder.Decode(report[:n])); err != nil {
			return fmt.Errorf("uinput: %w", err)
		}
	}
}

// checkHidraw refuses a node that is not a hidraw device of deviceId,
// read from its HID_ID in sysfs.
func checkHidraw(device *os.File, deviceID string) error {
	info, err := device.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("%s is not a character device", device.Name())
	}

	sysfs := fmt.Sprintf("/sys/dev/char/%d:%d", unixMajor(stat.Rdev), unixMinor(stat.Rdev))
	if link, err := filepath.EvalSymlinks(filepath.Join(sysfs, "subsystem")); err != nil || filepath.Base(link) != "hidraw" {
		return fmt.Errorf("%s is not a hidraw device", device.Name())
	}

	uevent, err := os.ReadFile(filepath.Join(sysfs, "device", "uevent"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(uevent), "\n") {
		// HID_ID=0003:000003F0:00000A56
		if id, found := strings.CutPrefix(line, "HID_ID="); found {
			parts := strings.Split(id, ":")
			if len(parts) == 3 && hidIDMatches(parts[1], parts[2], deviceID) {
				return nil
			}
			return fmt.Errorf("%s is HID %s, not %s", device.Name(), id, deviceID)
		}
	}

	return fmt.Errorf("%s has no HID_ID", device.Name())
}

func hidIDMatches(vendor, product, deviceID string) bool {
	wantVendor, wantProduct := splitDeviceID(deviceID)
	gotVendor, err1 := strconv.ParseUint(vendor, 16, 32)
	gotProduct, err2 := strconv.ParseUint(product, 16, 32)

	return err1 == nil && err2 == nil &&
		uint64(wantVendor) == gotVendor && uint64(wantProduct) == gotProduct
}

// splitDeviceID splits "vvvv:pppp"; a malformed ID gives zeros.
func splitDeviceID(deviceID string) (uint16, uint16) {
	vendorHex, productHex, _ := strings.Cut(deviceID, ":")
	vendor, _ := strconv.ParseUint(vendorHex, 16, 16)
	product, _ := strconv.ParseUint(productHex, 16, 16)

	return uint16(vendor), uint16(product)
}

// unixMajor and unixMinor split a Linux dev_t.
func unixMajor(dev uint64) uint64 { return (dev>>8)&0xfff | (dev>>32)&^0xfff }
func unixMinor(dev uint64) uint64 { return dev&0xff | (dev>>12)&^0xff }

// writePreset replaces dir/preset with the preset's name in one step, so
// a reader never sees a half-written file.
func writePreset(dir, preset string) error {
	temporary := filepath.Join(dir, ".preset")
	if err := os.WriteFile(temporary, []byte(preset+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(dir, "preset"))
}
