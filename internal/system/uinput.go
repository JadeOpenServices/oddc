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

	evSyn      = 0x00
	evKey      = 0x01
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

// keyboard is a uinput device that sends the keys it was created with.
type keyboard struct {
	file *os.File
}

func newKeyboard(path, name string, vendor, product uint16, codes []uint16) (*keyboard, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	k := &keyboard{file: file}

	if err := k.ioctl(uiSetEvBit, evKey); err != nil {
		k.Close()
		return nil, fmt.Errorf("uinput: %w", err)
	}
	for _, code := range codes {
		if err := k.ioctl(uiSetKeyBit, uintptr(code)); err != nil {
			k.Close()
			return nil, fmt.Errorf("uinput key %d: %w", code, err)
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
