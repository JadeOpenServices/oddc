// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"errors"
	"fmt"
	"sort"
)

// RenderRule is what a model's enabled render quirk says: the GPU (a key
// under hardware.graphics) that renders while only its own screens are in
// use. A compositor draws every screen on one GPU and copies each frame to
// screens of another, so the cheap choice is the GPU that drives the screens
// in use.
type RenderRule struct {
	Quirk   string
	Default string
}

// RenderRuleFrom finds the resolved model's enabled quirk with a renderGpu
// section. ok is false when the model enables none: then nothing chooses a
// render GPU for it.
func RenderRuleFrom(resolved map[string]any) (RenderRule, bool, error) {
	quirks, _ := resolved["quirks"].(map[string]any)
	keys := make([]string, 0, len(quirks))
	for key := range quirks {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var found []RenderRule
	for _, key := range keys {
		quirk, _ := quirks[key].(map[string]any)
		if quirk["enabled"] != true {
			continue
		}
		section, ok := quirk["renderGpu"].(map[string]any)
		if !ok {
			continue
		}
		def, _ := section["default"].(string)
		if def == "" {
			return RenderRule{}, false, fmt.Errorf("quirks.%s.renderGpu names no default GPU", key)
		}
		if _, ok := Lookup(resolved, "hardware.graphics."+def+".driver"); !ok {
			return RenderRule{}, false, fmt.Errorf("quirks.%s.renderGpu.default %q is not a GPU with a driver under hardware.graphics", key, def)
		}
		found = append(found, RenderRule{Quirk: key, Default: def})
	}
	switch len(found) {
	case 0:
		return RenderRule{}, false, nil
	case 1:
		return found[0], true, nil
	default:
		return RenderRule{}, false, errors.New("the model enables more than one renderGpu quirk")
	}
}

// GraphicsDrivers maps each GPU key under hardware.graphics to its driver.
func GraphicsDrivers(resolved map[string]any) map[string]string {
	drivers := map[string]string{}
	graphics, _ := Lookup(resolved, "hardware.graphics")
	gpus, _ := graphics.(map[string]any)
	for key, value := range gpus {
		gpu, _ := value.(map[string]any)
		if driver, _ := gpu["driver"].(string); driver != "" {
			drivers[key] = driver
		}
	}
	return drivers
}

// RenderGPU picks the GPU to render on: the one driving a connected screen
// that is not the default GPU's, else the default. inUse holds the GPU keys
// that drive at least one connected screen. More than one other GPU in use
// is an error: the rule does not say which of them wins.
func (r RenderRule) RenderGPU(inUse map[string]bool) (string, error) {
	var others []string
	for gpu, used := range inUse {
		if used && gpu != r.Default {
			others = append(others, gpu)
		}
	}
	sort.Strings(others)
	switch len(others) {
	case 0:
		return r.Default, nil
	case 1:
		return others[0], nil
	default:
		return "", fmt.Errorf("screens are in use on %v; the render rule picks one GPU besides %s", others, r.Default)
	}
}
