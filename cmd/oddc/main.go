package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JadeOpenServices/oddc"
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

// systemRoot is where the NixOS module deploys the selected model.
const systemRoot = "/etc/oddc"

func has(
	args []string,
	name string,
) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}

	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// deployedModel returns the model a deployed root was built for, or "".
func deployedModel(root string) string {
	data, err := os.ReadFile(
		filepath.Join(root, "resolved.json"),
	)
	if err != nil {
		return ""
	}

	var resolved struct {
		Model struct {
			ID string `json:"id"`
		} `json:"model"`
	}
	if json.Unmarshal(data, &resolved) != nil {
		return ""
	}

	return resolved.Model.ID
}

// withDefaults completes args for a deployed system: the root defaults to
// the system root when it holds a deployment, else the working directory;
// resolve and explain default to the deployed model and its host overlay.
func withDefaults(
	args []string,
	system string,
) []string {
	result := append([]string{}, args...)

	if !has(result, "--root") {
		root := "."
		if deployedModel(system) != "" {
			root = system
		}

		result = append(result, "--root", root)
	}

	if args[0] != "resolve" && args[0] != "explain" {
		return result
	}

	root := value(result, "--root", ".")
	model := deployedModel(root)

	if !has(result, "--device") && model != "" {
		result = append(result, "--device", model)
	}

	overlay := filepath.Join(root, "host-overlay.json")
	if !has(result, "--host") &&
		value(result, "--device", "") == model &&
		exists(overlay) {
		result = append(result, "--host", overlay)
	}

	return result
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(
			"usage: oddc <detect|setup|fetch|doctor|update|validate|list|resolve|explain>",
		)
	}

	switch args[0] {
	case "detect":
		return runDetect(args)
	case "setup":
		return runSetup(args)
	case "fetch":
		return runFetch(args)
	case "doctor":
		return runDoctor(args)
	case "update":
		return runUpdate(args)
	}

	args = withDefaults(args, systemRoot)

	root := value(
		args,
		"--root",
		".",
	)

	if args[0] == "validate" {
		return runValidate(root, has(args, "--json"))
	}

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		kind := value(
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

// gitRevision is the commit checked out at root, else fallback.
func gitRevision(root, fallback string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return fallback
	}

	return strings.TrimSpace(string(out))
}

// runValidate fails when the catalog is invalid; with --json it prints the
// full result either way.
func runValidate(root string, asJSON bool) error {
	result := oddc.Validate(root)
	if result.Revision == "local" {
		result.Revision = gitRevision(root, result.Revision)
	}

	if asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}

	if !result.Valid {
		return errors.New(strings.Join(result.Errors, "; "))
	}

	if !asJSON {
		fmt.Printf(
			"PASS: ODDC v2 entity registry valid (%d entities, %d evidence)\n",
			result.Entities,
			result.Evidence,
		)
	}

	return nil
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
