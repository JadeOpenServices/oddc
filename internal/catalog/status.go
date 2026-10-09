// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunStatus prints how far --device, else every model, is verified: the
// evidence its status rests on, what that evidence tested and its results.
// With --since REV it reports the models whose closure differs from the
// one at REV of the repository. With --since or --verified it fails unless
// every model it reports is verified.
func RunStatus(registry *oddc.Registry, args []string) error {
	models := cli.Values(args, "--device")
	since := cli.Value(args, "--since", "")
	if since != "" {
		var err error
		if models, err = changedSince(registry, since); err != nil {
			return err
		}
	} else if len(models) == 0 {
		for id, entity := range registry.Entities {
			if entity.Kind == "DeviceModel" {
				models = append(models, id)
			}
		}
		sort.Strings(models)
	}

	statuses := []oddc.Verification{}
	for _, model := range models {
		status, err := registry.Verify(model)
		if err != nil {
			return err
		}
		statuses = append(statuses, status)
	}

	var unproven []string
	for _, status := range statuses {
		if status.Status != oddc.Verified {
			unproven = append(unproven, status.Model+" is "+string(status.Status))
		}
	}
	var failure error
	if since != "" && len(unproven) > 0 {
		failure = fmt.Errorf("changed since %s, but not verified: %s", since, strings.Join(unproven, ", "))
	} else if cli.Has(args, "--verified") && len(unproven) > 0 {
		failure = fmt.Errorf("not verified: %s", strings.Join(unproven, ", "))
	}

	if cli.Has(args, "--json") {
		data, err := json.MarshalIndent(statuses, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return failure
	}

	for _, status := range statuses {
		fmt.Printf("%s: %s\n", status.Model, status.Status)
		if record := status.Evidence; record != nil {
			fmt.Printf("  evidence  %s (%s, %s)\n", record.ID, record.Status, record.ObservedAt)
			fmt.Printf("  tested    oddc %v, bios %v, kernel %v\n",
				orNone(record.Environment["oddc"]), orNone(record.Environment["bios"]), orNone(record.Environment["kernel"]))
			passed, other := Results(*record)
			fmt.Printf("  passed    %s\n", strings.Join(passed, ", "))
			if len(other) > 0 {
				fmt.Printf("  other     %s\n", strings.Join(other, ", "))
			}
		}
	}

	return failure
}

// changedSince lists the models of registry, a git checkout, whose closure
// differs from the one at revision since, new models included, sorted.
func changedSince(registry *oddc.Registry, since string) ([]string, error) {
	commit, err := cli.Git(registry.Root, "rev-parse", "--verify", "--end-of-options", since+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("%s is not a revision of %s: %w", since, registry.Root, err)
	}

	// The entities and schemas at since, without its evidence: closures
	// need none, and older evidence may not meet today's rules.
	dir, err := os.MkdirTemp("", "oddc-since-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out, err := cli.Git(registry.Root, "ls-tree", "-r", "-z", "--name-only", commit, "--", "catalog", "schemas")
	if err != nil {
		return nil, err
	}
	for _, file := range cli.NulSeparated(out) {
		data, err := cli.Git(registry.Root, "show", commit+":"+file)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(dir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(data+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	before, err := oddc.LoadRegistry(dir)
	if err != nil {
		return nil, fmt.Errorf("catalog at %s: %w", since, err)
	}

	var changed []string
	for id, entity := range registry.Entities {
		if entity.Kind != "DeviceModel" {
			continue
		}
		now, err := registry.Closure(id)
		if err != nil {
			return nil, err
		}
		if then, err := before.Closure(id); err != nil || then != now {
			changed = append(changed, id)
		}
	}
	sort.Strings(changed)

	return changed, nil
}

// Results splits a record's results into the names that passed and the
// others as NAME=STATUS, each sorted.
func Results(record oddc.Evidence) (passed, other []string) {
	for name, value := range record.Results {
		if value == "pass" {
			passed = append(passed, name)
		} else {
			other = append(other, fmt.Sprintf("%s=%v", name, value))
		}
	}
	sort.Strings(passed)
	sort.Strings(other)

	return passed, other
}

func orNone(value any) any {
	if value == nil {
		return "unrecorded"
	}
	return value
}
