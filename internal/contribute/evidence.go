package contribute

import (
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

// deployedRevision is the ODDC revision the NixOS module deployed, or "".
func deployedRevision() string {
	data, _ := os.ReadFile(filepath.Join(cli.SystemRoot, "revision"))
	return strings.TrimSpace(string(data))
}

func kernelRelease() string {
	data, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return kernelVersion.FindString(strings.TrimSpace(string(data)))
}

// RunEvidence records a new evidence file for this machine's model, or
// --device, in the workspace. Records are only ever added.
func RunEvidence(args []string) error {
	if len(args) < 2 || args[1] != "record" {
		return errors.New("usage: oddc evidence record [--device ID] [--result NAME=STATUS]... [--status STATUS] [--bios VERSION] [--revision COMMIT]")
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
	// What was tested: the BIOS, the ODDC revision deployed on this
	// machine, and the drivers bound to the model's components.
	sys := cli.Value(args, "--sys", "/sys")
	if bios := cli.Value(args, "--bios", oddc.ReadBIOS(sys)); bios != "" {
		environment["bios"] = bios
	}
	if revision := cli.Value(args, "--revision", deployedRevision()); revision != "" {
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
		Schema:        schemaRef(root, file, "evidence.schema.json"),
		SchemaVersion: oddc.SchemaVersion,
		ID:            path.Base(device) + "-" + name,
		DeviceID:      device,
		ObservedAt:    date,
		Status:        status,
		Environment:   environment,
		Results:       results,
		Drivers:       drivers,
	}
	if err := writeChecked(root, file, record); err != nil {
		return err
	}

	fmt.Printf("Recorded %s.\nSend it with `oddc contribute`.\n", file)
	return nil
}
