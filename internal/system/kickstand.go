// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// From include/uapi/linux/input.h.
const (
	eviocGrab = 0x40044590 // _IOW('E', 0x90, int)
	eviocGSw  = 0x8008451b // _IOC(_IOC_READ, 'E', 0x1b, 8): switch states
)

// switchCodes are the switch names a kickstand entry may use.
var switchCodes = map[string]uint16{"SW_TABLET_MODE": swTablet}

// kickstand reads a firmware switch that reports the stand. It holds the
// device exclusively, so only the combined tablet mode switch reaches the
// desktop.
type kickstand struct {
	file   *os.File
	code   uint16
	closed int32
}

// openKickstand opens the stand's switch: node when given (as the NixOS
// module names it), else the first input device found. Either way the
// device must belong to the ACPI device the model names (its sysfs parent
// is ACPIID:nn) and have the switch; then it is grabbed.
func openKickstand(sys string, stand oddc.Kickstand, node string) (*kickstand, error) {
	code, ok := switchCodes[stand.Switch]
	if !ok {
		return nil, fmt.Errorf("kickstand: unknown switch %q", stand.Switch)
	}

	path := ""
	if node != "" {
		real, err := filepath.EvalSymlinks(node)
		if err != nil {
			return nil, fmt.Errorf("kickstand: %w", err)
		}
		if !isKickstand(filepath.Join(sys, "class", "input", filepath.Base(real)), stand.ACPIID, code) {
			return nil, fmt.Errorf("kickstand: %s is not the %s switch of %s", node, stand.Switch, stand.ACPIID)
		}
		path = node
	} else {
		events, _ := filepath.Glob(filepath.Join(sys, "class", "input", "event*"))
		for _, event := range events {
			if isKickstand(event, stand.ACPIID, code) {
				path = filepath.Join("/dev/input", filepath.Base(event))
				break
			}
		}
		if path == "" {
			return nil, fmt.Errorf("kickstand: no input device of %s with %s", stand.ACPIID, stand.Switch)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	k := &kickstand{file: file, code: code, closed: int32(stand.ClosedValue)}
	if err := k.ioctl(eviocGrab, 1); err != nil {
		file.Close()
		return nil, fmt.Errorf("kickstand: grab %s: %w", path, err)
	}

	return k, nil
}

// isKickstand reports whether the input device at a sysfs event path
// belongs to the ACPI device acpiID and has the switch code.
func isKickstand(event, acpiID string, code uint16) bool {
	parent, err := filepath.EvalSymlinks(filepath.Join(event, "device", "device"))
	if err != nil || !strings.HasPrefix(filepath.Base(parent), acpiID+":") {
		return false
	}

	return hasSwitch(filepath.Join(event, "device", "capabilities", "sw"), code)
}

// hasSwitch reads an input device's switch bitmap, a hex number in sysfs.
func hasSwitch(path string, code uint16) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	bits, err := strconv.ParseUint(strings.TrimSpace(string(data)), 16, 64)

	return err == nil && bits&(1<<code) != 0
}

func (k *kickstand) ioctl(request, argument uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, k.file.Fd(), request, argument); errno != 0 {
		return errno
	}
	return nil
}

// Closed reads the switch's current state.
func (k *kickstand) Closed() (bool, error) {
	var states [8]byte
	if err := k.ioctl(eviocGSw, uintptr(unsafe.Pointer(&states[0]))); err != nil {
		return false, err
	}
	on := states[k.code/8]&(1<<(k.code%8)) != 0

	return on == (k.closed != 0), nil
}

// Watch sends the stand's state on every change of its switch, until the
// device fails; then it closes the channel.
func (k *kickstand) Watch(changes chan<- bool) {
	defer close(changes)

	buffer := make([]byte, 24*16)
	for {
		n, err := k.file.Read(buffer)
		if err != nil {
			return
		}
		for i := 0; i+24 <= n; i += 24 {
			kind := binary.NativeEndian.Uint16(buffer[i+16:])
			code := binary.NativeEndian.Uint16(buffer[i+18:])
			value := int32(binary.NativeEndian.Uint32(buffer[i+20:]))
			if kind == evSw && code == k.code {
				changes <- value == k.closed
			}
		}
	}
}

func (k *kickstand) Close() error {
	_ = k.ioctl(eviocGrab, 0)
	return k.file.Close()
}
