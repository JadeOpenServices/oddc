// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"errors"
	"math"
)

// Panel is a model's built-in display (hardware.display.internal.primary):
// its native resolution, physical size and refresh rate.
type Panel struct {
	ID                string
	Width, Height     int
	WidthMM, HeightMM int
	RefreshHz         int
}

// PanelFrom reads the model's built-in panel; ok is false when the model
// names none.
func PanelFrom(resolved map[string]any) (Panel, bool, error) {
	object, found := Lookup(resolved, "hardware.display.internal.primary")
	panel, isMap := object.(map[string]any)
	if !found || !isMap {
		return Panel{}, false, nil
	}
	id, _ := panel["id"].(string)
	p := Panel{ID: id}
	var err error
	get := func(path ...string) int {
		value, e := intField(nested(panel, path[:len(path)-1]), path[len(path)-1])
		if e != nil && err == nil {
			err = e
		}
		return value
	}
	p.Width = get("resolution", "width")
	p.Height = get("resolution", "height")
	p.WidthMM = get("size", "widthMm")
	p.HeightMM = get("size", "heightMm")
	p.RefreshHz = get("refreshHz")
	if err != nil {
		return Panel{}, false, err
	}
	if p.Width <= 0 || p.Height <= 0 || p.WidthMM <= 0 || p.HeightMM <= 0 {
		return Panel{}, false, errors.New("panel needs a positive resolution and size")
	}
	return p, true, nil
}

func nested(object map[string]any, path []string) map[string]any {
	for _, key := range path {
		object, _ = object[key].(map[string]any)
	}
	return object
}

// PPI is the panel's pixel density along its width.
func (p Panel) PPI() float64 {
	return float64(p.Width) / (float64(p.WidthMM) / 25.4)
}

// Scales lists the output scales from 1 to 3 at which the panel's logical
// size is a whole number of pixels in both directions, smallest first:
// the scales compositors such as Hyprland accept without correcting them.
// They are width/n for the n that divide both width and height into equal
// steps: n = (width/g)*k for the greatest common divisor g.
func (p Panel) Scales() []float64 {
	g := gcd(p.Width, p.Height)
	step := p.Width / g
	var scales []float64
	for n := step; n <= p.Width; n += step {
		scale := float64(p.Width) / float64(n)
		if scale < 1 {
			break
		}
		if scale <= 3 {
			scales = append([]float64{round6(scale)}, scales...)
		}
	}
	return scales
}

// CleanScales are the valid scales that are whole eighths, the ones worth
// offering in a settings menu.
func (p Panel) CleanScales() []float64 {
	var clean []float64
	for _, scale := range p.Scales() {
		if eighths := scale * 8; math.Abs(eighths-math.Round(eighths)) < 1e-6 {
			clean = append(clean, scale)
		}
	}
	return clean
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}
