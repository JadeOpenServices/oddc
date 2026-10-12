// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"math"
	"slices"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// The ZBook x2 G4's 3840x2160 panel: Hyprland rejected 1.78 and picked
// 1.875 on the machine; 1.875, 1.777778 and 2 must be valid, 1.78 not.
func TestPanelScales(t *testing.T) {
	registry, _ := fixture.Catalog(t)
	resolved, err := registry.ResolveModel("model/hp/zbook-x2-g4", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	panel, ok, err := PanelFrom(resolved.Resolved)
	if err != nil || !ok {
		t.Fatalf("no panel: %v", err)
	}
	if panel.ID != "display/lgd/05a4" || panel.Width != 3840 || panel.Height != 2160 {
		t.Fatalf("panel %+v", panel)
	}

	scales := panel.Scales()
	for _, want := range []float64{1, 1.5, 1.6, 1.666667, 1.777778, 1.875, 2, 3} {
		if !slices.Contains(scales, want) {
			t.Errorf("valid scale %v missing", want)
		}
	}
	if slices.Contains(scales, 1.78) || slices.Contains(scales, 1.88) {
		t.Error("1.78 or 1.88 listed as valid")
	}
	for _, scale := range scales {
		w, h := 3840/scale, 2160/scale
		if math.Abs(w-math.Round(w)) > 0.01 || math.Abs(h-math.Round(h)) > 0.01 {
			t.Errorf("scale %v gives %vx%v", scale, w, h)
		}
	}

	if got := panel.CleanScales(); !slices.Equal(got, []float64{1, 1.25, 1.5, 1.875, 2, 2.5, 3}) {
		t.Errorf("clean scales %v", got)
	}

	if ppi := panel.PPI(); ppi < 315 || ppi > 317 {
		t.Errorf("ppi %.1f, want 316", ppi)
	}
}
