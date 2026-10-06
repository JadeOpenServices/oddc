package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JadeOpenServices/oddc"
)

const upstream = "github:JadeOpenServices/oddc"

// workspaceDir is the default local catalog checkout.
func workspaceDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "oddc")
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "oddc")
}

// catalogRoot finds a full catalog: an explicit --root, the local
// workspace, else the upstream channel fetched into the Nix store.
func catalogRoot(args []string) (string, error) {
	if root := value(args, "--root", ""); root != "" {
		return root, nil
	}

	if dir := workspaceDir(); exists(filepath.Join(dir, "catalog")) {
		return dir, nil
	}

	ref := upstream
	if channel := value(args, "--channel", "main"); channel != "main" {
		ref += "/" + channel
	}

	output, err := exec.Command(
		"nix", "flake", "prefetch", "--json", ref,
	).Output()
	if err != nil {
		return "", fmt.Errorf("fetch catalog %s: %w", ref, err)
	}

	var prefetched struct {
		StorePath string `json:"storePath"`
	}
	if err := json.Unmarshal(output, &prefetched); err != nil {
		return "", err
	}

	return prefetched.StorePath, nil
}

func describe(identity oddc.MachineIdentity) string {
	return fmt.Sprintf(
		"vendor %q, product %q, board %q, form factor %q",
		identity.SysVendor,
		identity.ProductName,
		identity.BoardName,
		identity.FormFactor,
	)
}

// detect matches this machine against a full catalog.
func detect(args []string) (*oddc.Registry, string, error) {
	root, err := catalogRoot(args)
	if err != nil {
		return nil, "", err
	}

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		return nil, "", err
	}

	identity := oddc.ReadIdentity(value(args, "--sys", "/sys"))

	model, err := registry.MatchModel(identity)
	if errors.Is(err, oddc.ErrNoModelMatch) {
		return nil, "", fmt.Errorf(
			"no ODDC model matches this machine (%s); "+
				"add it with `oddc scaffold` and `oddc contribute`",
			describe(identity),
		)
	}
	if err != nil {
		return nil, "", err
	}

	return registry, model, nil
}

func runDetect(args []string) error {
	registry, model, err := detect(args)
	if err != nil {
		return err
	}

	fmt.Printf(
		"%s\t%s\n",
		model,
		registry.Entities[model].Metadata.Name,
	)

	return nil
}

func runSetup(args []string) error {
	registry, model, err := detect(args)
	if err != nil {
		return err
	}

	if deployed := deployedModel(systemRoot); deployed == model {
		fmt.Printf("Already set up: %s\n", model)
		return nil
	}

	fmt.Printf(`# This machine: %s

# flake.nix
inputs.oddc.url = "%s";

# NixOS configuration (pass inputs to your modules)
{
  imports = [ inputs.oddc.nixosModules.default ];
  oddc.device = "%s";
}
`,
		registry.Entities[model].Metadata.Name,
		upstream,
		model,
	)

	return nil
}

// runDoctor checks a deployment against this machine without network.
func runDoctor(args []string) error {
	root := value(args, "--root", systemRoot)
	failed := false

	report := func(status, format string, a ...any) {
		if status == "FAIL" {
			failed = true
		}

		fmt.Printf("%s: %s\n", status, fmt.Sprintf(format, a...))
	}

	model := deployedModel(root)
	if model == "" {
		report("FAIL", "no ODDC deployment in %s; see `oddc setup`", root)
		return errors.New("doctor found problems")
	}
	report("PASS", "deployed model %s", model)

	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		report("FAIL", "deployment invalid: %v", err)
	} else {
		identity := oddc.ReadIdentity(value(args, "--sys", "/sys"))

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

func command(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// runUpdate takes the newest catalog into a system flake and, with
// --switch, rebuilds.
func runUpdate(args []string) error {
	flake := value(args, "--flake", "/etc/nixos")

	lock, err := os.ReadFile(filepath.Join(flake, "flake.lock"))
	if err != nil {
		return fmt.Errorf("%s is not a locked flake: %w", flake, err)
	}

	var parsed struct {
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(lock, &parsed); err != nil {
		return err
	}
	if _, ok := parsed.Nodes["oddc"]; !ok {
		return fmt.Errorf("%s has no oddc input", flake)
	}

	if err := command(
		"nix", "flake", "update", "oddc", "--flake", flake,
	); err != nil {
		return err
	}

	if !has(args, "--switch") {
		fmt.Printf(
			"Updated oddc. Apply with: sudo nixos-rebuild switch --flake %s\n",
			flake,
		)
		return nil
	}

	return command(
		"sudo", "nixos-rebuild", "switch", "--flake", flake,
	)
}
