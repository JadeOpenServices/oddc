// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunDevices prints every component of a model (--device ID, default the
// deployed one; --root DIR, default the deployment): role path, entity,
// name, bus, device ID, attachment, fixed USB port, whether it is usable on
// Linux, what the model's evidence proves of it, and the protocol a tool
// speaks to it. It is the one inventory consumers read: a USB trust broker
// takes the devices with a usbPort, for example. --json prints JSON.
//
// With no model deployed or named, it classifies this machine (--sys,
// default /sys, or --facts FILE) against the catalog at --root. A match is
// printed as above. With no match the model is null and it prints what the
// installer needs for a best-effort setup: every part present, the GPUs
// with the driver bound now and the experimental options proven for the
// same chip on other models, the graphics-safety parameters the live
// system booted with (--proc, default /proc), and how to contribute the
// machine.
func RunDevices(args []string) error {
	root := cli.Value(args, "--root", cli.SystemRoot)
	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return err
	}
	model := cli.Value(args, "--device", cli.DeployedModel(root))
	if model == "" {
		facts, err := cli.ReadFacts(args)
		if err != nil {
			return err
		}
		classified, err := registry.Classify(facts)
		if err != nil {
			return err
		}
		switch classified.Result {
		case oddc.ResultMatched:
			model = classified.Model
		case oddc.ResultAmbiguous:
			return fmt.Errorf("this machine matches more than one model: %s", strings.Join(classified.Ambiguous, ", "))
		default:
			return bestEffort(args, registry, facts)
		}
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

// bestEffort prints what RunDevices knows of a machine no model matches.
// Nothing in it is proven for this machine, and nothing in it is applied:
// the installer sets up the base and shows this.
func bestEffort(args []string, registry *oddc.Registry, facts oddc.Facts) error {
	sys := cli.Value(args, "--sys", "/sys")
	if cli.Value(args, "--facts", "") != "" {
		sys = ""
	}
	var bound map[string]map[string][]string
	var gpus []oddc.GPU
	if sys != "" {
		bound = oddc.ReadDrivers(sys)
		gpus = oddc.ReadGPUs(sys)
	}
	gpus, err := registry.GraphicsOptions(gpus)
	if err != nil {
		return err
	}
	if gpus == nil {
		gpus = []oddc.GPU{}
	}
	parts := registry.PresentParts(facts, bound)
	live := oddc.LiveBoot{Parameters: []string{}}
	if sys != "" {
		cmdline, _ := os.ReadFile(filepath.Join(cli.Value(args, "--proc", "/proc"), "cmdline"))
		live.Parameters = oddc.SafeGraphicsParameters(string(cmdline))
	}
	contribute := oddc.ContributeInfo()

	if cli.Has(args, "--json") {
		out := json.NewEncoder(os.Stdout)
		out.SetIndent("", "  ")
		return out.Encode(map[string]any{
			"model":      nil,
			"devices":    parts,
			"graphics":   gpus,
			"liveBoot":   live,
			"contribute": contribute,
		})
	}

	fmt.Println("No ODDC model matches this machine: best effort, nothing here is proven on it.")
	for _, p := range parts {
		name := p.Name
		if p.ID == "" {
			name = "not in the catalog"
		}
		if !p.Supported {
			name += ", unusable on Linux: " + p.UnsupportedReason
		}
		fmt.Printf("%-5s %s %s\n", p.Bus, p.DeviceID, name)
	}
	for _, g := range gpus {
		driver := g.Driver
		if driver == "" {
			driver = "no driver"
		}
		fmt.Printf("graphics %s %s: %s, %s\n", g.Slot, g.DeviceID, driver, g.Mode)
		for _, o := range g.Options {
			fmt.Printf("  experimental: driver branch %s, tested on %s (%s, kernel %s)\n", o.DriverBranch, o.ProvenOn.Name, o.ProvenOn.OS, o.ProvenOn.Kernel)
		}
	}
	if len(live.Parameters) > 0 {
		fmt.Printf("The live system booted with %s; the installed system needs the same.\n", strings.Join(live.Parameters, " "))
	}
	fmt.Printf("Add this machine to ODDC: %s (%s)\n", strings.Join(contribute.Commands, ", then "), contribute.Guide)
	if contribute.ReportEndpoint != nil {
		fmt.Printf("Or send this report to %s\n", *contribute.ReportEndpoint)
	}
	return nil
}
