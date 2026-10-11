// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunDisplay prints the resolved model's built-in panel as JSON: its
// resolution, size, density and the output scales it accepts, for
// compositors and monitor tools to use instead of guessing; which scale to
// default to is theirs to choose (--resolved, default /etc/oddc/resolved.json).
func RunDisplay(args []string) error {
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
	panel, ok, err := oddc.PanelFrom(resolved)
	switch {
	case err != nil:
		return err
	case !ok:
		return errors.New("the resolved model names no built-in panel")
	}

	connector, _ := oddc.Lookup(resolved, "hardware.display.internal.primary.connector")
	out := map[string]any{
		"id":          panel.ID,
		"connector":   connector,
		"width":       panel.Width,
		"height":      panel.Height,
		"widthMm":     panel.WidthMM,
		"heightMm":    panel.HeightMM,
		"refreshHz":   panel.RefreshHz,
		"ppi":         int(panel.PPI() + 0.5),
		"scales":      panel.Scales(),
		"cleanScales": panel.CleanScales(),
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}
