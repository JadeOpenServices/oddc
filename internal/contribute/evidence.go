// SPDX-License-Identifier: GPL-3.0-or-later

package contribute

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

var (
	evidenceStatuses = []string{
		"documented", "detected", "configured", "runtime-verified", "hardware-validated",
	}

	// kernelVersion keeps the upstream version of a kernel release and
	// drops a local suffix, which can name a machine.
	kernelVersion = regexp.MustCompile(`^\d+\.\d+(?:\.\d+)?`)
)

// osName is NAME and VERSION_ID from os-release, such as "NixOS 25.11".
func osName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}

	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			fields[key] = strings.Trim(value, `"'`)
		}
	}

	return strings.TrimSpace(fields["NAME"] + " " + fields["VERSION_ID"])
}

// deployment is what the NixOS module deployed at root for device: its
// ODDC revision, the closure digest and the quirks it did not apply. All
// are empty when root holds no deployment of device.
func deployment(root, device string) (revision, closure string, inactive []string, err error) {
	if cli.DeployedModel(root) != device {
		return "", "", nil, nil
	}

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return "", "", nil, fmt.Errorf("deployment %s: %w", root, err)
	}
	if closure, err = registry.Closure(device); err != nil {
		return "", "", nil, err
	}

	if data, err := os.ReadFile(filepath.Join(root, "inactive-quirks.json")); err == nil {
		if err := json.Unmarshal(data, &inactive); err != nil {
			return "", "", nil, fmt.Errorf("deployment %s: inactive-quirks.json: %w", root, err)
		}
	}

	data, _ := os.ReadFile(filepath.Join(root, "revision"))
	return strings.TrimSpace(string(data)), closure, inactive, nil
}

func kernelRelease() string {
	data, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return kernelVersion.FindString(strings.TrimSpace(string(data)))
}

// RunEvidence records a new evidence file for this machine's model, or
// --device, in the workspace. Records are only ever added.
func RunEvidence(args []string) error {
	if len(args) < 2 || args[1] != "record" {
		return errors.New("usage: oddc evidence record [--device ID] [--result NAME=STATUS]... [--status STATUS] [--bios VERSION] [--deployment DIR]")
	}

	root, registry, err := loadWorkspace(args)
	if err != nil {
		return err
	}

	facts, err := cli.ReadFacts(args)
	if err != nil {
		return err
	}

	classification, err := registry.Classify(facts)
	if err != nil {
		return err
	}

	device := cli.Value(args, "--device", classification.Model)
	if device == "" {
		return errors.New("no model matches this machine; name it with --device, or draft one with `oddc scaffold`")
	}

	results := map[string]any{}
	if classification.Result == oddc.ResultMatched && classification.Model == device {
		results["identity"] = "pass"
	}
	for _, result := range cli.Values(args, "--result") {
		name, status, ok := strings.Cut(result, "=")
		if !ok {
			return fmt.Errorf("--result %q is not NAME=STATUS", result)
		}
		results[name] = status
	}
	if len(results) == 0 {
		return errors.New("nothing to record; add --result NAME=STATUS")
	}

	status := cli.Value(args, "--status", "detected")
	if !slices.Contains(evidenceStatuses, status) {
		return fmt.Errorf("--status %q is not one of %s", status, strings.Join(evidenceStatuses, ", "))
	}

	environment := map[string]any{}
	if name := cli.Value(args, "--os", osName()); name != "" {
		environment["os"] = name
	}
	if kernel := cli.Value(args, "--kernel", kernelRelease()); kernel != "" {
		environment["kernel"] = kernel
	}
	// What was tested: the BIOS, the ODDC revision, closure and inactive
	// quirks deployed on this machine, and the drivers bound to the model's components.
	sys := cli.Value(args, "--sys", "/sys")
	if bios := cli.Value(args, "--bios", oddc.ReadBIOS(sys)); bios != "" {
		environment["bios"] = bios
	}
	revision, closure, inactive, err := deployment(cli.Value(args, "--deployment", cli.SystemRoot), device)
	if err != nil {
		return err
	}
	if revision != "" {
		environment["oddc"] = revision
	}
	drivers := map[string][]string{}
	if !cli.Has(args, "--facts") {
		if drivers, err = registry.ComponentDrivers(device, oddc.ReadDrivers(sys)); err != nil {
			return err
		}
	}

	date := cli.Value(args, "--date", time.Now().UTC().Format(time.DateOnly))
	dir := filepath.Join(root, "evidence", filepath.FromSlash(device))
	name := date
	for n := 2; cli.Exists(filepath.Join(dir, name+".json")); n++ {
		name = fmt.Sprintf("%s-%d", date, n)
	}
	file := filepath.Join(dir, name+".json")

	record := oddc.Evidence{
		Schema:         schemaRef(root, file, "evidence.schema.json"),
		SchemaVersion:  oddc.SchemaVersion,
		ID:             path.Base(device) + "-" + name,
		DeviceID:       device,
		ObservedAt:     date,
		Status:         status,
		Environment:    environment,
		Results:        results,
		Drivers:        drivers,
		Closure:        closure,
		InactiveQuirks: inactive,
	}
	// Passing evidence must prove the model the workspace holds.
	if status == "runtime-verified" || status == "hardware-validated" {
		current, err := registry.Closure(device)
		if err != nil {
			return err
		}
		if closure != "" && closure != current {
			return fmt.Errorf("the deployed %s is not the one in %s; record from a checkout of the deployed revision %s", device, root, revision)
		}
		unmet, err := registry.Unmet(device, record)
		if err != nil {
			return err
		}
		if len(unmet) > 0 {
			return fmt.Errorf("this does not prove %s %s:\n  %s", device, status, strings.Join(unmet, "\n  "))
		}
	}
	if err := writeChecked(root, file, record); err != nil {
		return err
	}

	fmt.Printf("Recorded %s.\nSend it with `oddc contribute`.\n", file)
	return nil
}
