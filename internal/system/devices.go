// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunDevices prints every component of a model (--device ID, default the
// deployed one; --root DIR, default the deployment): role path, entity,
// name, bus, device ID, attachment, fixed USB port, whether it is usable on
// Linux, what the model's evidence proves of it, and the protocol a tool
// speaks to it. It is the one inventory consumers read: a USB trust broker
// takes the devices with a usbPort, for example. --json prints JSON.
func RunDevices(args []string) error {
	root := cli.Value(args, "--root", cli.SystemRoot)
	model := cli.Value(args, "--device", cli.DeployedModel(root))
	if model == "" {
		return errors.New("name the model with --device")
	}
	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return err
	}
	devices, err := registry.Devices(model)
	if err != nil {
		return err
	}
	if cli.Has(args, "--json") {
		out := json.NewEncoder(os.Stdout)
		out.SetIndent("", "  ")
		return out.Encode(map[string]any{"model": model, "devices": devices})
	}
	for _, d := range devices {
		where := d.Attachment
		if d.USBPort != nil {
			where = fmt.Sprintf("%s, %s %s port %d", where, d.USBPort.Controller, d.USBPort.Hub, d.USBPort.Port)
		}
		if where != "" {
			where = " (" + where + ")"
		}
		fmt.Printf("%-40s %-5s %s %s%s, proven: %s\n", d.Path, d.Bus, d.DeviceID, d.Name, where, d.Proven)
	}
	return nil
}
