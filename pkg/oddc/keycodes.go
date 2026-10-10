// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import "fmt"

// KeyCodes are the Linux key names a keymap may use, with their codes from
// include/uapi/linux/input-event-codes.h.
var KeyCodes = func() map[string]uint16 {
	codes := map[string]uint16{
		"KEY_ESC":                1,
		"KEY_MUTE":               113,
		"KEY_VOLUMEDOWN":         114,
		"KEY_VOLUMEUP":           115,
		"KEY_LEFTMETA":           125,
		"KEY_PROG1":              148,
		"KEY_PROG2":              149,
		"KEY_SCREENLOCK":         152,
		"KEY_ROTATE_DISPLAY":     153,
		"KEY_NEXTSONG":           163,
		"KEY_PLAYPAUSE":          164,
		"KEY_PREVIOUSSONG":       165,
		"KEY_PROG3":              202,
		"KEY_PROG4":              203,
		"KEY_PRINT":              210,
		"KEY_BRIGHTNESSDOWN":     224,
		"KEY_BRIGHTNESSUP":       225,
		"KEY_MICMUTE":            248,
		"KEY_KEYBOARD":           0x176,
		"KEY_ROTATE_LOCK_TOGGLE": 0x231,
	}
	for n := 13; n <= 24; n++ {
		codes[fmt.Sprintf("KEY_F%d", n)] = uint16(183 + n - 13)
	}
	for n := 1; n <= 30; n++ {
		codes[fmt.Sprintf("KEY_MACRO%d", n)] = uint16(0x290 + n - 1)
	}

	return codes
}()
