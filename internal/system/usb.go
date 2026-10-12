// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunUSB prints the deployed model's built-in USB devices (--resolved,
// default /etc/oddc/resolved.json): each with its role path, name, device
// ID, controller PCI ID, root hub, port, connect type, attachment and
// whether it is usable on Linux. A USB trust broker reads them here
// instead of walking the resolved model itself. --json prints JSON.
func RunUSB(args []string) error {
	data, err := os.ReadFile(cli.Value(args, "--resolved", "/etc/oddc/resolved.json"))
	if err != nil {
		return err
	}
	var resolved map[string]any
	if err := json.Unmarshal(data, &resolved); err != nil {
		return err
	}
	devices, err := oddc.InternalUSBDevices(resolved)
	if err != nil {
		return err
	}
	if devices == nil {
		devices = []oddc.InternalUSBDevice{}
	}
	if cli.Has(args, "--json") {
		out := json.NewEncoder(os.Stdout)
		out.SetIndent("", "  ")
		return out.Encode(map[string]any{"devices": devices})
	}
	for _, d := range devices {
		usable := ""
		if !d.Supported {
			usable = ", not usable on Linux"
		}
		fmt.Printf("%s %s on %s %s port %d (%s, %s%s): %s\n", d.DeviceID, d.Path, d.Controller, d.Hub, d.Port, d.ConnectType, d.Attachment, usable, d.Name)
	}
	return nil
}
