// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunStatus prints how far --device, else every model, is verified: the
// evidence its status rests on, what that evidence tested and its results.
func RunStatus(registry *oddc.Registry, args []string) error {
	models := cli.Values(args, "--device")
	if len(models) == 0 {
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

	if cli.Has(args, "--json") {
		data, err := json.MarshalIndent(statuses, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return nil
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

	return nil
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
