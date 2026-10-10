// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// From include/uapi/linux/uinput.h and input.h.
const (
	uiDevCreate  = 0x5501     // _IO('U', 1)
	uiDevDestroy = 0x5502     // _IO('U', 2)
	uiDevSetup   = 0x405c5503 // _IOW('U', 3, struct uinput_setup)
	uiSetEvBit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeyBit  = 0x40045565 // _IOW('U', 101, int)
	uiSetSwBit   = 0x4004556d // _IOW('U', 109, int)

	evSyn      = 0x00
	evKey      = 0x01
	evSw       = 0x05
	swTablet   = 0x01
	synReport  = 0
	busVirtual = 0x06
)

// uinputSetup is struct uinput_setup: struct input_id, an 80 byte name
// and ff_effects_max.
type uinputSetup struct {
	Bustype, Vendor, Product, Version uint16
	Name                              [80]byte
	FFEffectsMax                      uint32
}

// keyboard is a uinput device that sends the keys or switches it was
// created with.
type keyboard struct {
	file *os.File
}

func newKeyboard(path, name string, vendor, product uint16, codes []uint16) (*keyboard, error) {
	return newUinput(path, name, vendor, product, evKey, uiSetKeyBit, codes)
}

// newTabletSwitch creates a device with one switch, SW_TABLET_MODE.
func newTabletSwitch(path, name string) (*keyboard, error) {
	return newUinput(path, name, 0, 0, evSw, uiSetSwBit, []uint16{swTablet})
}

func newUinput(path, name string, vendor, product uint16, kind uint16, setBit uintptr, codes []uint16) (*keyboard, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	k := &keyboard{file: file}

	if err := k.ioctl(uiSetEvBit, uintptr(kind)); err != nil {
		k.Close()
		return nil, fmt.Errorf("uinput: %w", err)
	}
	for _, code := range codes {
		if err := k.ioctl(setBit, uintptr(code)); err != nil {
			k.Close()
			return nil, fmt.Errorf("uinput code %d: %w", code, err)
		}
	}

	setup := uinputSetup{Bustype: busVirtual, Vendor: vendor, Product: product, Version: 1}
	copy(setup.Name[:len(setup.Name)-1], name)
	if err := k.ioctl(uiDevSetup, uintptr(unsafe.Pointer(&setup))); err != nil {
		k.Close()
		return nil, fmt.Errorf("uinput setup: %w", err)
	}
	if err := k.ioctl(uiDevCreate, 0); err != nil {
		k.Close()
		return nil, fmt.Errorf("uinput create: %w", err)
	}

	return k, nil
}

func (k *keyboard) ioctl(request, argument uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, k.file.Fd(), request, argument); errno != 0 {
		return errno
	}
	return nil
}

// Send writes the events and one SYN_REPORT after them.
func (k *keyboard) Send(events []oddc.KeyEvent) error {
	if len(events) == 0 {
		return nil
	}

	var buffer []byte
	for _, event := range events {
		value := int32(0)
		if event.Pressed {
			value = 1
		}
		buffer = appendInputEvent(buffer, evKey, event.Code, value)
	}
	buffer = appendInputEvent(buffer, evSyn, synReport, 0)

	_, err := k.file.Write(buffer)
	return err
}

// SetSwitch sets one switch and reports it.
func (k *keyboard) SetSwitch(code uint16, on bool) error {
	value := int32(0)
	if on {
		value = 1
	}
	buffer := appendInputEvent(nil, evSw, code, value)
	buffer = appendInputEvent(buffer, evSyn, synReport, 0)

	_, err := k.file.Write(buffer)
	return err
}

// appendInputEvent appends struct input_event: a 16 byte timeval the
// kernel fills in, type, code and value.
func appendInputEvent(buffer []byte, kind, code uint16, value int32) []byte {
	buffer = append(buffer, make([]byte, 16)...)
	buffer = binary.NativeEndian.AppendUint16(buffer, kind)
	buffer = binary.NativeEndian.AppendUint16(buffer, code)
	return binary.NativeEndian.AppendUint32(buffer, uint32(value))
}

func (k *keyboard) Close() error {
	_ = k.ioctl(uiDevDestroy, 0)
	return k.file.Close()
}
