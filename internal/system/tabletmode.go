// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// attachSettle is how long a returning keyboard must stay before laptop
// mode: keyboards bounce on their connector and drop off again.
const attachSettle = 2 * time.Second

// RunTabletMode keeps a virtual SW_TABLET_MODE switch in step with the
// model: on while none of its detachable keyboards is on its bus, off once
// one has been back for attachSettle. It holds the stand's own switch
// (capabilities.kickstandSwitch), which the firmware reports as
// SW_TABLET_MODE, so the desktop sees only this one. It reads the model from --resolved (default
// /etc/oddc/resolved.json), the devices from --sys and the stand's switch
// from --kickstand (default: found under --sys).
func RunTabletMode(args []string) error {
	data, err := os.ReadFile(cli.Value(args, "--resolved", "/etc/oddc/resolved.json"))
	if err != nil {
		return err
	}
	var resolved map[string]any
	if err := json.Unmarshal(data, &resolved); err != nil {
		return err
	}
	keyboards := oddc.TabletModeKeyboards(resolved)
	if len(keyboards) == 0 {
		return errors.New("the resolved model has no keyboard whose presence signals tablet mode")
	}
	sys := cli.Value(args, "--sys", "/sys")

	events, err := listenUevents()
	if err != nil {
		return fmt.Errorf("uevents: %w", err)
	}
	defer syscall.Close(events)

	if stand, ok := oddc.KickstandSwitch(resolved); ok {
		switchDevice, err := openKickstand(sys, stand, cli.Value(args, "--kickstand", ""))
		if err != nil {
			return err
		}
		defer switchDevice.Close()
	}

	device, err := newTabletSwitch(cli.Value(args, "--uinput", "/dev/uinput"), "ODDC Tablet Mode Switch")
	if err != nil {
		return err
	}
	defer device.Close()

	tablet := oddc.TabletMode(keyboards, oddc.ReadDevices(sys))
	if err := device.SetSwitch(swTablet, tablet); err != nil {
		return err
	}

	devicesChanges := make(chan struct{}, 1)
	go watchUevents(events, devicesChanges)

	var settle <-chan time.Time
	for {
		settled := false
		select {
		case _, open := <-devicesChanges:
			if !open {
				return errors.New("uevent socket closed")
			}
		case <-settle:
			settle, settled = nil, true
		}

		now := oddc.TabletMode(keyboards, oddc.ReadDevices(sys))
		switch {
		case now == tablet:
			settle = nil
			continue
		case !now && !settled:
			// Laptop mode only once it holds: keyboards bounce on their
			// connector.
			if settle == nil {
				settle = time.After(attachSettle)
			}
			continue
		}

		tablet, settle = now, nil
		if err := device.SetSwitch(swTablet, tablet); err != nil {
			return fmt.Errorf("uinput: %w", err)
		}
	}
}

// watchUevents signals every USB or HID device added or removed, and
// closes changes when the socket fails.
func watchUevents(events int, changes chan<- struct{}) {
	defer close(changes)

	buffer := make([]byte, 8192)
	for {
		n, err := syscall.Read(events, buffer)
		if err != nil {
			return
		}
		if devicesChanged(buffer[:n]) {
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	}
}

// listenUevents opens the kernel's uevent multicast group.
func listenUevents() (int, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM|syscall.SOCK_CLOEXEC, syscall.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return -1, err
	}
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: 1}); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	return fd, nil
}

// devicesChanged reports whether a uevent adds or removes a USB or HID
// device: "ACTION@devpath\0KEY=value\0...".
func devicesChanged(message []byte) bool {
	fields := strings.Split(string(message), "\x00")
	action, subsystem := "", ""
	for _, field := range fields {
		if value, ok := strings.CutPrefix(field, "ACTION="); ok {
			action = value
		}
		if value, ok := strings.CutPrefix(field, "SUBSYSTEM="); ok {
			subsystem = value
		}
	}

	return (action == "add" || action == "remove") && (subsystem == "usb" || subsystem == "hid")
}
