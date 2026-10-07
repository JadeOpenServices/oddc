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
