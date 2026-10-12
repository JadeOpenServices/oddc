// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunDoctor checks a deployment against this machine without network.
func RunDoctor(args []string) error {
	root := cli.Value(args, "--root", cli.SystemRoot)
	failed := false

	report := func(status, format string, a ...any) {
		if status == "FAIL" {
			failed = true
		}

		fmt.Printf("%s: %s\n", status, fmt.Sprintf(format, a...))
	}

	model := cli.DeployedModel(root)
	if model == "" {
		report("FAIL", "no ODDC deployment in %s; see `oddc setup`", root)
		return errors.New("doctor found problems")
	}
	report("PASS", "deployed model %s", model)

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		report("FAIL", "deployment invalid: %v", err)
	} else {
		identity := oddc.ReadIdentity(cli.Value(args, "--sys", "/sys"))

		if matched, err := registry.MatchModel(identity); err != nil ||
			matched != model {
			report(
				"FAIL",
				"this machine (%s) does not match %s",
				describe(identity),
				model,
			)
		} else {
			report("PASS", "hardware identity matches")
		}

		if resolved, err := registry.ResolveEntity(model); err == nil {
			for _, part := range oddc.UnsupportedComponents(resolved.Resolved) {
				report("INFO", "%s (%s) cannot be used on Linux: %s", part.Name, part.Path, part.Reason)
			}
		}

		verification(registry, model, oddc.ReadBIOS(cli.Value(args, "--sys", "/sys")), report)
	}

	if data, err := os.ReadFile(
		filepath.Join(root, "revision"),
	); err == nil {
		report("PASS", "ODDC revision %s", strings.TrimSpace(string(data)))
	} else {
		report("WARN", "no ODDC revision recorded; import the module through the flake")
	}

	if failed {
		return errors.New("doctor found problems")
	}

	return nil
}

// verification reports whether the deployed closure is proven by evidence
// and whether this machine runs the BIOS that evidence was recorded on.
// Neither fails: a laptop on another BIOS or an unproven closure still
// deploys, and the user sees what is known.
func verification(registry *oddc.Registry, model, bios string, report func(status, format string, a ...any)) {
	status, err := registry.Verify(model)
	if err != nil {
		report("FAIL", "verification: %v", err)
		return
	}

	record := status.Evidence
	switch status.Status {
	case oddc.Verified:
		report("PASS", "verified by evidence %s", record.ID)
	case oddc.Changed:
		report("WARN", "changed since evidence %s; record new evidence with `oddc evidence record`", record.ID)
	default:
		report("WARN", "unverified: no passing evidence names this closure; record it with `oddc evidence record`")
	}
	if record == nil {
		return
	}

	tested, _ := record.Environment["bios"].(string)
	switch {
	case tested == "" || bios == "":
		report("WARN", "BIOS not compared: evidence %s or this machine does not name it", record.ID)
	case tested != bios:
		report("WARN", "BIOS %s differs from %s that evidence %s was recorded on", bios, tested, record.ID)
	default:
		report("PASS", "BIOS %s as evidence %s", bios, record.ID)
	}
}
