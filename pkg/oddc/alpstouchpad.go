// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// AlpsTouchpad is an Alps touchpad chip behind a keyboard controller that
// passes I2C-HID commands through one vendor feature report
// (touchpad.protocol "alps-u1-report-passthrough" on a keyboard component).
// The chip speaks the Alps U1 register protocol: a mode register switches
// it between mouse reports and absolute multitouch reports.
type AlpsTouchpad struct {
	FeatureReport byte
	InputReport   byte
	ModeRegister  uint32
	MouseMode     byte
	AbsoluteMode  byte
	MaxContacts   int
	XMax, YMax    int
	WidthMM       int
	HeightMM      int
	// ReportLength per transport ("usb", "bluetooth").
	ReportLength map[string]int
	// Writable per transport: whether the keyboard accepts SET_FEATURE.
	Writable map[string]bool
}

const alpsProtocol = "alps-u1-report-passthrough"

// AlpsTouchpadFrom reads the touchpad of the model's primary keyboard.
// ok is false when the keyboard has no such touchpad.
func AlpsTouchpadFrom(resolved map[string]any) (AlpsTouchpad, bool, error) {
	object, found := Lookup(resolved, "hardware.input.keyboard.primary.touchpad")
	touchpad, isMap := object.(map[string]any)
	if !found || !isMap || touchpad["protocol"] != alpsProtocol {
		return AlpsTouchpad{}, false, nil
	}

	var t AlpsTouchpad
	var err error
	field := func(name string) int {
		value, e := intField(touchpad, name)
		if e != nil && err == nil {
			err = e
		}
		return value
	}
	t.FeatureReport = byte(field("featureReport"))
	t.InputReport = byte(field("inputReport"))
	t.ModeRegister = uint32(field("modeRegister"))
	t.MouseMode = byte(field("mouseMode"))
	t.AbsoluteMode = byte(field("absoluteMode"))
	t.MaxContacts = field("maxContacts")
	t.XMax = field("xMax")
	t.YMax = field("yMax")
	t.WidthMM = field("widthMm")
	t.HeightMM = field("heightMm")
	if err != nil {
		return AlpsTouchpad{}, false, err
	}

	t.ReportLength = map[string]int{}
	t.Writable = map[string]bool{}
	transports, _ := touchpad["transports"].(map[string]any)
	for name, value := range transports {
		transport, _ := value.(map[string]any)
		length, e := intField(transport, "reportLength")
		if e != nil {
			return AlpsTouchpad{}, false, fmt.Errorf("touchpad transport %s: %w", name, e)
		}
		t.ReportLength[name] = length
		t.Writable[name], _ = transport["writable"].(bool)
	}

	if t.MaxContacts < 1 || t.MaxContacts > 5 || t.XMax <= 0 || t.YMax <= 0 || t.WidthMM <= 0 || t.HeightMM <= 0 {
		return AlpsTouchpad{}, false, errors.New("touchpad needs maxContacts 1-5 and positive xMax, yMax, widthMm, heightMm")
	}
	if len(t.ReportLength) == 0 {
		return AlpsTouchpad{}, false, errors.New("touchpad names no transports")
	}
	return t, true, nil
}

// intField accepts a JSON number or a "0x..." string.
func intField(object map[string]any, name string) (int, error) {
	switch value := object[name].(type) {
	case float64:
		return int(value), nil
	case int:
		return value, nil
	case interface{ Int64() (int64, error) }:
		n, err := value.Int64()
		return int(n), err
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(value), 0, 64)
		return int(n), err
	}
	return 0, fmt.Errorf("touchpad field %q missing or not a number", name)
}

// u1Packet is one Alps U1 register command: report id 5, command
// (0xD1 read, 0xD2 write), the 32-bit address little endian, the value
// and a checksum of 0x0A plus the other bytes.
func u1Packet(command byte, address uint32, value byte) []byte {
	packet := []byte{0x05, command, byte(address), byte(address >> 8), byte(address >> 16), byte(address >> 24), value}
	sum := byte(0x0A)
	for _, b := range packet {
		sum += b
	}
	return append(packet, sum)
}

// The I2C-HID header in front of a U1 packet: command register 0x0005,
// SET_REPORT (0x35 0x03) for feature report 6 on data register 0x0006,
// length 0x000A.
var alpsSetReport = []byte{0x05, 0x00, 0x35, 0x03, 0x06, 0x00, 0x0A, 0x00}

func (t AlpsTouchpad) report(length int, body ...byte) []byte {
	report := make([]byte, length)
	report[0] = t.FeatureReport
	copy(report[1:], body)
	return report
}

// StatusRequest asks the keyboard whether the touchpad is ready.
func (t AlpsTouchpad) StatusRequest(length int) []byte {
	return t.report(length, 0x03, 0x02, 0x02)
}

// StatusReady reads the reply to StatusRequest.
func StatusReady(reply []byte) bool {
	return len(reply) > 2 && reply[2] == 0xC4
}

// ModeWrite sets the mode register to value.
func (t AlpsTouchpad) ModeWrite(length int, value byte) []byte {
	body := append([]byte{0x01, 0x10}, alpsSetReport...)
	return t.report(length, append(body, u1Packet(0xD2, t.ModeRegister, value)...)...)
}

// ModeReadRequest and ModeReadFetch read the mode register; the value
// comes back in the next GET of the feature report.
func (t AlpsTouchpad) ModeReadRequest(length int) []byte {
	body := append([]byte{0x01, 0x10}, alpsSetReport...)
	return t.report(length, append(body, u1Packet(0xD1, t.ModeRegister, 0)...)...)
}

func (t AlpsTouchpad) ModeReadFetch(length int) []byte {
	return t.report(length, 0x02, 0x06, 0x05, 0x00, 0x35, 0x02, 0x06, 0x00)
}

// ModeReadValue reads the reply to ModeReadFetch; ok is false without
// the chip's acknowledgement (0xD7).
func ModeReadValue(reply []byte) (byte, bool) {
	if len(reply) < 9 || reply[3] != 0xD7 {
		return 0, false
	}
	return reply[8], true
}

// Contact is one finger in an absolute report: its slot (0-4), position
// and pressure.
type Contact struct {
	Slot     int
	X, Y     int
	Pressure int
}

// TouchFrame is one absolute report: the fingers on the pad and whether
// the pad is pressed down.
type TouchFrame struct {
	Contacts []Contact
	Button   bool
}

// ParseFrame reads one absolute input report: byte 1 bit 0 the button,
// byte 2 bit 7 set for a frame to drop, then five slots of x, y (16 bit
// little endian) and pressure; an empty slot reads 0xFFFF. ok is false
// for other reports, dropped frames and the keyboard's filler reports
// (bytes 0x7F, 0x7E, 0x7D, ...).
func (t AlpsTouchpad) ParseFrame(report []byte) (TouchFrame, bool) {
	if len(report) < 3+5*t.MaxContacts || report[0] != t.InputReport {
		return TouchFrame{}, false
	}
	if report[1] == 0x7F && report[2] == 0x7E && report[3] == 0x7D {
		return TouchFrame{}, false
	}
	if report[2]&0x80 != 0 {
		return TouchFrame{}, false
	}

	frame := TouchFrame{Button: report[1]&0x01 != 0}
	for slot := 0; slot < t.MaxContacts; slot++ {
		o := 3 + slot*5
		x := int(report[o]) | int(report[o+1])<<8
		y := int(report[o+2]) | int(report[o+3])<<8
		if x == 0xFFFF || y == 0xFFFF {
			continue
		}
		frame.Contacts = append(frame.Contacts, Contact{Slot: slot, X: x, Y: y, Pressure: int(report[o+4] & 0x7F)})
	}
	return frame, true
}
