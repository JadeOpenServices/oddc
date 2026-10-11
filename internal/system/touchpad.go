// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// From include/uapi/linux/uinput.h and input-event-codes.h.
const (
	uiSetAbsBit  = 0x40045567 // _IOW('U', 103, int)
	uiSetPropBit = 0x4004556e // _IOW('U', 110, int)
	uiAbsSetup   = 0x401c5504 // _IOW('U', 4, struct uinput_abs_setup)

	hidiocSFeature = 0xC0004806 // _IOWR('H', 0x06, len), length added below
	hidiocGFeature = 0xC0004807 // _IOWR('H', 0x07, len)

	evAbs = 0x03

	absX, absY, absPressure                 = 0x00, 0x01, 0x18
	absMTSlot, absMTX, absMTY               = 0x2f, 0x35, 0x36
	absMTTrackingID, absMTPressure          = 0x39, 0x3a
	btnLeft, btnTouch                       = 0x110, 0x14a
	btnToolFinger, btnToolQuintTap          = 0x145, 0x148
	btnToolDoubleTap, btnToolTripleTap      = 0x14d, 0x14e
	btnToolQuadTap                          = 0x14f
	propPointer, propButtonpad              = 0x00, 0x02
	mouseReport                        byte = 0x05
)

// RunTouchpad turns an Alps touchpad's absolute reports into a Linux
// multitouch touchpad on a uinput device, following the resolved model's
// keyboard (touchpad.protocol "alps-u1-report-passthrough"). --device is
// the keyboard's hidraw node for --transport usb or bluetooth. Where the
// transport accepts writes, it switches the touchpad to absolute mode
// and back to mouse mode when it stops; where it doesn't, it reads a mode
// set earlier. Input devices the generic HID driver made from the
// absolute report are grabbed so nothing else misreads it.
func RunTouchpad(args []string) error {
	data, err := os.ReadFile(cli.Value(args, "--resolved", "/etc/oddc/resolved.json"))
	if err != nil {
		return err
	}
	var resolved map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&resolved); err != nil {
		return err
	}

	touchpad, ok, err := oddc.AlpsTouchpadFrom(resolved)
	switch {
	case err != nil:
		return err
	case !ok:
		return errors.New("the resolved model's keyboard has no Alps touchpad")
	}

	transport := cli.Value(args, "--transport", "usb")
	length, known := touchpad.ReportLength[transport]
	if !known {
		return fmt.Errorf("the touchpad has no transport %q", transport)
	}
	writable := touchpad.Writable[transport]

	deviceID, bus, err := keyboardIdentity(resolved, transport)
	if err != nil {
		return err
	}
	device, err := os.OpenFile(cli.Value(args, "--device", "/dev/oddc/touchpad-"+transport), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer device.Close()
	if err := checkHidraw(device, deviceID); err != nil {
		return err
	}
	if err := checkHidBus(device, bus); err != nil {
		return err
	}

	if writable {
		if err := setAbsolute(device, touchpad, length); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "touchpad: absolute mode on over %s\n", transport)
	} else {
		fmt.Fprintf(os.Stderr, "touchpad: %s accepts no writes; reading the mode set over another transport\n", transport)
	}

	pad, err := newTouchpad(cli.Value(args, "--uinput", "/dev/uinput"), touchpad)
	if err != nil {
		if writable {
			_ = setMode(device, touchpad, length, touchpad.MouseMode)
		}
		return err
	}
	defer pad.Close()

	grabbed := grabAbsoluteInputs(device)
	fmt.Fprintf(os.Stderr, "touchpad: \"ODDC Alps Touchpad\" created; %d misread input device(s) grabbed\n", len(grabbed))
	defer func() {
		for _, file := range grabbed {
			file.Close()
		}
	}()

	// Mouse mode again on stop, so the pad keeps working without us.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		if writable {
			_ = setMode(device, touchpad, length, touchpad.MouseMode)
		}
		pad.Close()
		os.Exit(0)
	}()

	lastFix := time.Time{}
	frames, reported := 0, time.Now()
	// Frames in a row with a finger down; the notice goes out on the third,
	// as the Alps driver package does it.
	touching := 0
	report := make([]byte, 64)
	for {
		n, err := device.Read(report)
		if err != nil {
			return fmt.Errorf("reading %s: %w", device.Name(), err)
		}
		if n > 0 && report[0] == mouseReport {
			// The pad fell back to mouse mode (power loss, resume).
			if writable && time.Since(lastFix) > 2*time.Second {
				lastFix = time.Now()
				if err := setAbsolute(device, touchpad, length); err == nil {
					fmt.Fprintln(os.Stderr, "touchpad: was back in mouse mode; absolute mode on again")
				}
			}
			continue
		}
		if frame, ok := touchpad.ParseFrame(report[:n]); ok {
			if err := pad.Send(frame); err != nil {
				return fmt.Errorf("uinput: %w", err)
			}
			if len(frame.Contacts) == 0 {
				touching = 0
			} else if touching++; touching == 3 && writable {
				_ = feature(device, hidiocSFeature, touchpad.FingerNotice(length))
			}
			frames++
			if time.Since(reported) > time.Minute {
				fmt.Fprintf(os.Stderr, "touchpad: %d frames in the last minute\n", frames)
				frames, reported = 0, time.Now()
			}
		}
	}
}

// keyboardIdentity is the keyboard's HID ID and bus type on a transport:
// its own deviceId over USB, bluetooth.deviceId over Bluetooth.
func keyboardIdentity(resolved map[string]any, transport string) (string, string, error) {
	path, bus := "hardware.input.keyboard.primary.deviceId", "0003"
	if transport == "bluetooth" {
		path, bus = "hardware.input.keyboard.primary.bluetooth.deviceId", "0005"
	}
	value, _ := oddc.Lookup(resolved, path)
	id, ok := value.(string)
	if !ok || !strings.Contains(id, ":") {
		return "", "", fmt.Errorf("the keyboard has no %s", path)
	}
	return id, bus, nil
}

// checkHidBus refuses a hidraw node on another HID bus type.
func checkHidBus(device *os.File, bus string) error {
	sysfs, err := hidrawSysfs(device)
	if err != nil {
		return err
	}
	uevent, err := os.ReadFile(filepath.Join(sysfs, "device", "uevent"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(uevent), "HID_ID="+bus+":") {
		return fmt.Errorf("%s is not on HID bus %s", device.Name(), bus)
	}
	return nil
}

func hidrawSysfs(device *os.File) (string, error) {
	info, err := device.Stat()
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("%s: no device number", device.Name())
	}
	return fmt.Sprintf("/sys/dev/char/%d:%d", unixMajor(stat.Rdev), unixMinor(stat.Rdev)), nil
}

func feature(device *os.File, request uintptr, buffer []byte) error {
	request |= uintptr(len(buffer)) << 16
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, device.Fd(), request, uintptr(unsafe.Pointer(&buffer[0]))); errno != 0 {
		return errno
	}
	return nil
}

// setAbsolute waits for the touchpad to report ready (up to 5 tries, a
// second apart) and switches it to absolute mode.
func setAbsolute(device *os.File, t oddc.AlpsTouchpad, length int) error {
	ready := false
	for try := 0; try < 5 && !ready; try++ {
		if try > 0 {
			time.Sleep(time.Second)
		}
		if err := feature(device, hidiocSFeature, t.StatusRequest(length)); err != nil {
			continue
		}
		reply := make([]byte, length)
		reply[0] = t.FeatureReport
		if feature(device, hidiocGFeature, reply) == nil && oddc.StatusReady(reply) {
			ready = true
		}
	}
	if !ready {
		return errors.New("the touchpad did not report ready")
	}
	return setMode(device, t, length, t.AbsoluteMode)
}

// setMode writes the mode register and reads it back.
func setMode(device *os.File, t oddc.AlpsTouchpad, length int, mode byte) error {
	pause := func() { time.Sleep(10 * time.Millisecond) }
	if err := feature(device, hidiocSFeature, t.ModeWrite(length, mode)); err != nil {
		return fmt.Errorf("touchpad mode write: %w", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := feature(device, hidiocSFeature, t.ModeReadRequest(length)); err != nil {
		return fmt.Errorf("touchpad mode read: %w", err)
	}
	pause()
	if err := feature(device, hidiocSFeature, t.ModeReadFetch(length)); err != nil {
		return fmt.Errorf("touchpad mode read: %w", err)
	}
	pause()
	reply := make([]byte, length)
	reply[0] = t.FeatureReport
	if err := feature(device, hidiocGFeature, reply); err != nil {
		return fmt.Errorf("touchpad mode read: %w", err)
	}
	if got, ok := oddc.ModeReadValue(reply); !ok || got != mode {
		return fmt.Errorf("touchpad mode is %#x after writing %#x", got, mode)
	}
	return nil
}

// grabAbsoluteInputs grabs the input devices the generic HID driver made
// next to this hidraw node from the absolute report: those with only
// absolute axes (its vendor collection over USB), and those whose only key
// is KEY_MICMUTE (over Bluetooth the report sits on HP's vendor page
// 0xFF01 usage 1, which the kernel maps to the mic mute key). The keyboard
// and the mouse stay untouched.
func grabAbsoluteInputs(device *os.File) []*os.File {
	sysfs, err := hidrawSysfs(device)
	if err != nil {
		return nil
	}
	events, _ := filepath.Glob(filepath.Join(sysfs, "device", "input", "input*", "event*"))
	var grabbed []*os.File
	for _, event := range events {
		capabilities := filepath.Join(event, "device", "capabilities")
		ev, err := readBitmap(filepath.Join(capabilities, "ev"))
		if err != nil {
			continue
		}
		keys, _ := readBitmap(filepath.Join(capabilities, "key"))
		// EV_SYN and EV_ABS, perhaps EV_MSC; no keys, no relative axes.
		absoluteOnly := ev.has(evAbs) && ev.only(evSyn, evAbs, 0x04)
		// EV_SYN, EV_KEY, perhaps EV_MSC and EV_REP; the one key KEY_MICMUTE.
		micMuteOnly := ev.has(evKey) && ev.only(evSyn, evKey, 0x04, 0x14) && keys.has(keyMicMute) && keys.only(keyMicMute)
		if !absoluteOnly && !micMuteOnly {
			continue
		}
		file, err := os.Open(filepath.Join("/dev/input", filepath.Base(event)))
		if err != nil {
			continue
		}
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), eviocGrab, 1); errno != 0 {
			file.Close()
			continue
		}
		grabbed = append(grabbed, file)
	}
	return grabbed
}

// bitmap is an input capability bitmap from sysfs: hex words, most
// significant first, each word one unsigned long.
type bitmap []uint64

const keyMicMute = 248

func readBitmap(path string) (bitmap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	words := strings.Fields(string(data))
	b := make(bitmap, len(words))
	for i, word := range words {
		value, err := strconv.ParseUint(word, 16, 64)
		if err != nil {
			return nil, err
		}
		b[len(words)-1-i] = value
	}
	return b, nil
}

func (b bitmap) has(bit int) bool {
	return bit/64 < len(b) && b[bit/64]&(1<<(bit%64)) != 0
}

// only reports whether no bit outside allowed is set.
func (b bitmap) only(allowed ...int) bool {
	mask := make(bitmap, len(b))
	for _, bit := range allowed {
		if bit/64 < len(mask) {
			mask[bit/64] |= 1 << (bit % 64)
		}
	}
	for i, word := range b {
		if word&^mask[i] != 0 {
			return false
		}
	}
	return true
}

// touchpad is a uinput multitouch touchpad (a clickpad: one button under
// the whole surface).
type touchpad struct {
	*keyboard
	slots  []int32
	nextID int32
}

// uinputAbsSetup is struct uinput_abs_setup: the axis code, padding and
// struct input_absinfo.
type uinputAbsSetup struct {
	Code                                      uint16
	_                                         uint16
	Value, Minimum, Maximum, Fuzz, Flat, Resl int32
}

func newTouchpad(path string, t oddc.AlpsTouchpad) (*touchpad, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	k := &keyboard{file: file}
	fail := func(what string, err error) (*touchpad, error) {
		k.Close()
		return nil, fmt.Errorf("uinput %s: %w", what, err)
	}

	for _, kind := range []uintptr{evKey, evAbs} {
		if err := k.ioctl(uiSetEvBit, kind); err != nil {
			return fail("event type", err)
		}
	}
	for _, code := range []uintptr{btnLeft, btnTouch, btnToolFinger, btnToolDoubleTap, btnToolTripleTap, btnToolQuadTap, btnToolQuintTap} {
		if err := k.ioctl(uiSetKeyBit, code); err != nil {
			return fail("key", err)
		}
	}
	for _, prop := range []uintptr{propPointer, propButtonpad} {
		if err := k.ioctl(uiSetPropBit, prop); err != nil {
			return fail("property", err)
		}
	}

	xRes := int32(t.XMax / t.WidthMM)
	yRes := int32(t.YMax / t.HeightMM)
	axes := []uinputAbsSetup{
		{Code: absX, Maximum: int32(t.XMax), Resl: xRes},
		{Code: absY, Maximum: int32(t.YMax), Resl: yRes},
		{Code: absPressure, Maximum: 127},
		{Code: absMTSlot, Maximum: int32(t.MaxContacts - 1)},
		{Code: absMTX, Maximum: int32(t.XMax), Resl: xRes},
		{Code: absMTY, Maximum: int32(t.YMax), Resl: yRes},
		{Code: absMTTrackingID, Maximum: 65535},
		{Code: absMTPressure, Maximum: 127},
	}
	for _, axis := range axes {
		if err := k.ioctl(uiSetAbsBit, uintptr(axis.Code)); err != nil {
			return fail("axis", err)
		}
		axis := axis
		if err := k.ioctl(uiAbsSetup, uintptr(unsafe.Pointer(&axis))); err != nil {
			return fail("axis setup", err)
		}
	}

	setup := uinputSetup{Bustype: busVirtual, Version: 1}
	copy(setup.Name[:len(setup.Name)-1], "ODDC Alps Touchpad")
	if err := k.ioctl(uiDevSetup, uintptr(unsafe.Pointer(&setup))); err != nil {
		return fail("setup", err)
	}
	if err := k.ioctl(uiDevCreate, 0); err != nil {
		return fail("create", err)
	}

	slots := make([]int32, t.MaxContacts)
	for i := range slots {
		slots[i] = -1
	}
	return &touchpad{keyboard: k, slots: slots}, nil
}

// Send reports one frame: each slot's finger with a tracking ID that
// stays the same while the finger stays down, the finger count tools,
// the single-touch axes from the first finger, and the button.
func (p *touchpad) Send(frame oddc.TouchFrame) error {
	active := make(map[int]oddc.Contact, len(frame.Contacts))
	for _, contact := range frame.Contacts {
		active[contact.Slot] = contact
	}

	var buffer []byte
	add := func(kind, code uint16, value int32) {
		buffer = appendInputEvent(buffer, kind, code, value)
	}
	for slot := range p.slots {
		contact, down := active[slot]
		switch {
		case down:
			add(evAbs, absMTSlot, int32(slot))
			if p.slots[slot] < 0 {
				p.slots[slot] = p.nextID
				p.nextID = (p.nextID + 1) & 0xFFFF
				add(evAbs, absMTTrackingID, p.slots[slot])
			}
			add(evAbs, absMTX, int32(contact.X))
			add(evAbs, absMTY, int32(contact.Y))
			add(evAbs, absMTPressure, int32(contact.Pressure))
		case p.slots[slot] >= 0:
			add(evAbs, absMTSlot, int32(slot))
			add(evAbs, absMTTrackingID, -1)
			p.slots[slot] = -1
		}
	}

	count := len(frame.Contacts)
	press := func(on bool) int32 {
		if on {
			return 1
		}
		return 0
	}
	add(evKey, btnTouch, press(count > 0))
	add(evKey, btnToolFinger, press(count == 1))
	add(evKey, btnToolDoubleTap, press(count == 2))
	add(evKey, btnToolTripleTap, press(count == 3))
	add(evKey, btnToolQuadTap, press(count == 4))
	add(evKey, btnToolQuintTap, press(count >= 5))
	if count > 0 {
		first := frame.Contacts[0]
		add(evAbs, absX, int32(first.X))
		add(evAbs, absY, int32(first.Y))
		add(evAbs, absPressure, int32(first.Pressure))
	} else {
		add(evAbs, absPressure, 0)
	}
	add(evKey, btnLeft, press(frame.Button))
	add(evSyn, synReport, 0)

	_, err := p.file.Write(buffer)
	return err
}
