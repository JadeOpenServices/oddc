// SPDX-License-Identifier: GPL-3.0-or-later

// Package catalog holds the commands that read a catalog: validate, list,
// index, classify, resolve, explain, status and changes.
package catalog

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Run runs a catalog command over --root: by default the deployed
// system when there is one, else the working directory.
func Run(args []string) error {
	args = WithDefaults(args, cli.SystemRoot)

	root := cli.Value(
		args,
		"--root",
		".",
	)

	if args[0] == "validate" {
		return RunValidate(root, cli.Value(args, "--since", ""), cli.Has(args, "--json"))
	}

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		kind := cli.Value(
			args,
			"--kind",
			"",
		)

		ids := make([]string, 0, len(registry.Entities))
		for id, entity := range registry.Entities {
			if kind == "" || entity.Kind == kind {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)

		for _, id := range ids {
			entity := registry.Entities[id]
			fmt.Printf(
				"%s\t%s\t%s\n",
				id,
				entity.Kind,
				entity.Metadata.Name,
			)
		}

		return nil

	case "index":
		index, err := registry.Index(revision(root))
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return nil

	case "classify":
		return RunClassify(registry, args)

	case "status":
		return RunStatus(registry, args)

	case "resolve", "explain":
		modelID := cli.Value(
			args,
			"--device",
			"",
		)
		if modelID == "" {
			return fmt.Errorf(
				"--device is required",
			)
		}

		project, err := loadOverlays(
			cli.Values(args, "--project"),
		)
		if err != nil {
			return err
		}

		host, err := loadOverlays(
			cli.Values(args, "--host"),
		)
		if err != nil {
			return err
		}

		resolved, err := registry.ResolveModel(
			modelID,
			project,
			host,
		)
		if err != nil {
			return err
		}

		if args[0] == "resolve" {
			data, err := json.MarshalIndent(
				resolved,
				"",
				"  ",
			)
			if err != nil {
				return err
			}

			fmt.Println(string(data))
			return nil
		}

		path := cli.Value(
			args,
			"--path",
			"",
		)
		if path == "" {
			return fmt.Errorf(
				"--path is required",
			)
		}

		resolvedValue, exists := oddc.Lookup(
			resolved.Resolved,
			path,
		)
		if !exists {
			return fmt.Errorf(
				"unknown resolved path %q",
				path,
			)
		}

		fmt.Printf(
			"path: %s\n",
			path,
		)
		fmt.Printf(
			"value: %v\n",
			resolvedValue,
		)

		if current, exists :=
			resolved.Provenance[path]; exists {
			fmt.Printf(
				"source: %s\n",
				current.Source,
			)
		}

		if history := resolved.History[path]; len(history) > 1 {
			fmt.Println("history:")

			for _, item := range history {
				fmt.Printf(
					"  %s -> %v\n",
					item.Source,
					item.Value,
				)
			}
		}

		return nil

	default:
		return fmt.Errorf(
			"unknown command %q",
			args[0],
		)
	}
}
