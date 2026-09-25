package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bakanura/gjallarOS/pkg/oddc"
)

func value(
	args []string,
	name string,
	fallback string,
) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}

	return fallback
}

func values(
	args []string,
	name string,
) []string {
	var result []string

	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			result = append(
				result,
				args[i+1],
			)
		}
	}

	return result
}

func loadOverlays(
	paths []string,
) ([]oddc.Overlay, error) {
	result := make(
		[]oddc.Overlay,
		0,
		len(paths),
	)

	for _, path := range paths {
		overlay, err := oddc.ReadOverlay(path)
		if err != nil {
			return nil, err
		}

		result = append(
			result,
			overlay,
		)
	}

	return result, nil
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(
			"usage: oddcctl <validate|resolve|explain>",
		)
	}

	root := value(
		args,
		"--root",
		"./oddc",
	)

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return err
	}

	switch args[0] {
	case "validate":
		fmt.Printf(
			"PASS: ODDC v2 entity registry valid (%d entities)\n",
			len(registry.Entities),
		)

		return nil

	case "resolve", "explain":
		modelID := value(
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
			values(args, "--project"),
		)
		if err != nil {
			return err
		}

		host, err := loadOverlays(
			values(args, "--host"),
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

		path := value(
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

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(
			os.Stderr,
			"FAIL:",
			err,
		)
		os.Exit(1)
	}
}
