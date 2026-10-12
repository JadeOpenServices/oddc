// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func unhex(t *testing.T, text string) []byte {
	t.Helper()
	data, err := hex.DecodeString(strings.ReplaceAll(text, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// alpsTouchpadOf resolves the ZBook x2 G4 and reads its keyboard's touchpad.
func alpsTouchpadOf(t *testing.T) AlpsTouchpad {
	t.Helper()
	registry, _ := fixture.Catalog(t)
	resolved, err := registry.ResolveModel("model/hp/zbook-x2-g4", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	touchpad, ok, err := AlpsTouchpadFrom(resolved.Resolved)
	if err != nil || !ok {
		t.Fatalf("no Alps touchpad: %v", err)
	}
	return touchpad
}

// The commands are the bytes sent on the ZBook x2 G4 on 2026-10-11, as
// the Alps driver package sends them: 26-byte reports over Bluetooth,
// 48 bytes over USB, zero padded.
func TestAlpsTouchpadCommands(t *testing.T) {
	touchpad := alpsTouchpadOf(t)

	if touchpad.ReportLength["usb"] != 48 || touchpad.ReportLength["bluetooth"] != 26 {
		t.Fatalf("report lengths %v", touchpad.ReportLength)
	}
	if !touchpad.Writable["usb"] || touchpad.Writable["bluetooth"] {
		t.Fatalf("writable %v: USB accepts writes, Bluetooth refuses them", touchpad.Writable)
	}

	cases := []struct {
		name string
		got  []byte
		want string
	}{
		{"status", touchpad.StatusRequest(48), "09 03 02 02"},
		{"finger notice", touchpad.FingerNotice(48), "09 03 02 01 01"},
		{"read request", touchpad.ModeReadRequest(48), "09 01 10 05 00 35 03 06 00 0a 00 05 d1 40 00 80 00 00 a0"},
		{"read fetch", touchpad.ModeReadFetch(48), "09 02 06 05 00 35 02 06 00"},
		{"absolute", touchpad.ModeWrite(48, touchpad.AbsoluteMode), "09 01 10 05 00 35 03 06 00 0a 00 05 d2 40 00 80 00 22 c3"},
		{"mouse", touchpad.ModeWrite(48, touchpad.MouseMode), "09 01 10 05 00 35 03 06 00 0a 00 05 d2 40 00 80 00 21 c2"},
	}
	for _, c := range cases {
		want := unhex(t, c.want)
		if len(c.got) != 48 || !bytes.Equal(c.got[:len(want)], want) || bytes.ContainsAny(c.got[len(want):], "\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\xff") {
			t.Errorf("%s: %x, want %x zero padded to 48", c.name, c.got, want)
		}
	}
	if len(touchpad.Init) != 15 || touchpad.Init[0].Register != 0x800051 || touchpad.Init[1].PauseMs != 500 || touchpad.Init[0].Verify || !touchpad.Init[2].Verify {
		t.Errorf("init table %+v", touchpad.Init)
	}
	// The first and last init writes as the driver package sends them.
	for _, c := range []struct {
		w    RegisterWrite
		want string
	}{
		{touchpad.Init[0], "09 01 10 05 00 35 03 06 00 0a 00 05 d2 51 00 80 00 c1 73"},
		{touchpad.Init[14], "09 01 10 05 00 35 03 06 00 0a 00 05 d2 83 00 80 00 20 04"},
	} {
		if got, want := touchpad.RegisterSet(48, c.w.Register, c.w.Value), unhex(t, c.want); !bytes.Equal(got[:len(want)], want) {
			t.Errorf("init write %x, want %x", got[:len(want)], want)
		}
	}
	if got := touchpad.ModeWrite(26, touchpad.AbsoluteMode); len(got) != 26 {
		t.Errorf("Bluetooth length %d", len(got))
	}
}

// Replies the keyboard gave over USB.
func TestAlpsTouchpadReplies(t *testing.T) {
	if !StatusReady(unhex(t, "02 02 c4 00 00 00 00 00")) {
		t.Error("ready status not recognised")
	}
	if StatusReady(unhex(t, "02 02 00 00 00 00 00 00")) {
		t.Error("not-ready status taken as ready")
	}
	if value, ok := ModeReadValue(unhex(t, "0a 00 05 d7 40 00 80 00 21 00")); !ok || value != 0x21 {
		t.Errorf("mode read %#x %v, want 0x21", value, ok)
	}
	if _, ok := ModeReadValue(unhex(t, "9e 61 82 fb 37 9a 20 83 4e")); ok {
		t.Error("a reply without acknowledgement was accepted")
	}
}

// Frames captured over USB and Bluetooth in absolute mode, the keyboard's
// filler report and a mouse report.
func TestAlpsTouchpadFrames(t *testing.T) {
	touchpad := alpsTouchpadOf(t)

	frame, ok := touchpad.ParseFrame(unhex(t, "07 00 01 a1 05 42 06 51 ff ff ff ff 00 ff ff ff ff 00 ff ff ff ff 00 ff ff ff ff 00"))
	if !ok || len(frame.Contacts) != 1 || frame.Contacts[0] != (Contact{Slot: 0, X: 0x05a1, Y: 0x0642, Pressure: 0x51}) || frame.Button {
		t.Errorf("USB frame: %+v %v", frame, ok)
	}
	frame, ok = touchpad.ParseFrame(unhex(t, "07 00 01 05 05 e8 01 5e ff ff ff ff 00 ff ff ff ff 00 ff ff ff ff 00 ff ff ff ff 00"))
	if !ok || len(frame.Contacts) != 1 || frame.Contacts[0] != (Contact{Slot: 0, X: 0x0505, Y: 0x01e8, Pressure: 0x5e}) {
		t.Errorf("Bluetooth frame: %+v %v", frame, ok)
	}
	for _, other := range []string{
		"07 7f 7e 7d 7c 7b 7a 79 78 77 76 75 74 73 72 71 70 6f 6e 6d 6c 6b 6a 69 68 67 66 65",
		"05 00 01 ff 00 00",
	} {
		if _, ok := touchpad.ParseFrame(unhex(t, other)); ok {
			t.Errorf("%s parsed as a touch frame", other)
		}
	}
}
