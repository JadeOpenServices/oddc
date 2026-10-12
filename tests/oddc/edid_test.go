// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// The ZBook x2 G4's panel: the first 12 bytes of its EDID (header,
// manufacturer LGD, product 0x05A4) as read on the machine; the rest,
// serial number included, left zero.
func zbookPanelEDID() []byte {
	edid := make([]byte, 128)
	copy(edid, []byte{0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00, 0x30, 0xe4, 0xa4, 0x05})
	return edid
}

func TestEDIDPanelID(t *testing.T) {
	if got := EDIDPanelID(zbookPanelEDID()); got != "lgd:05a4" {
		t.Errorf("panel ID %q, want lgd:05a4", got)
	}
	if got := EDIDPanelID(zbookPanelEDID()[:64]); got != "" {
		t.Errorf("short EDID gave %q", got)
	}
	if got := EDIDPanelID(make([]byte, 128)); got != "" {
		t.Errorf("EDID without header gave %q", got)
	}
}

// Only built-in panels count; an external monitor's EDID is not a fact
// of the machine.
func TestReadPanelIDsBuiltInOnly(t *testing.T) {
	root := t.TempDir()
	external := zbookPanelEDID()
	external[10] = 0xa5
	for connector, edid := range map[string][]byte{
		"card1-eDP-1":    zbookPanelEDID(),
		"card0-eDP-2":    nil,
		"card1-HDMI-A-1": external,
	} {
		dir := filepath.Join(root, "class", "drm", connector)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "edid"), edid, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := ReadPanelIDs(root)
	if len(got) != 1 || got[0] != "lgd:05a4" {
		t.Errorf("panels %v, want [lgd:05a4] from eDP-1 only", got)
	}
}
