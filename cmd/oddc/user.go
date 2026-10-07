package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
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

// source finds the catalog: an explicit --root, the local workspace,
// else GitHub at --rev or the --channel branch. Reading from GitHub
// downloads only the files a command needs.
func source(args []string) (oddc.Source, error) {
	if root := value(args, "--root", ""); root != "" {
		return oddc.DirSource{Root: root}, nil
	}

	if dir := workspaceDir(); exists(filepath.Join(dir, "catalog")) {
		return oddc.DirSource{Root: dir}, nil
	}

	return oddc.NewGitHubSource(
		&http.Client{Timeout: time.Minute},
		oddc.GitHubAPI,
		oddc.GitHubRaw,
		oddc.Repository,
		value(args, "--rev", value(args, "--channel", "main")),
	)
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

func noMatch(identity oddc.MachineIdentity) error {
	return fmt.Errorf(
		"no ODDC model matches this machine (%s); "+
			"add it with `oddc scaffold` and `oddc contribute`",
		describe(identity),
	)
}

// detect fetches this machine's model and loads it.
func detect(args []string) (*oddc.Registry, string, error) {
	src, err := source(args)
	if err != nil {
		return nil, "", err
	}

	dir, err := os.MkdirTemp("", "oddc-detect-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(dir)

	identity := oddc.ReadIdentity(value(args, "--sys", "/sys"))
	answer := filepath.Join(dir, "answer")

	model, err := oddc.Fetch(src, identity, answer)
	if errors.Is(err, oddc.ErrNoModelMatch) {
		return nil, "", noMatch(identity)
	}
	if err != nil {
		return nil, "", err
	}

	registry, err := oddc.LoadRegistry(answer)
	if err != nil {
		return nil, "", err
	}

	return registry, model, nil
}

// runFetch writes only this machine's model, or the --device model, to
// --out: its reference closure, its evidence and the ODDC revision.
func runFetch(args []string) error {
	out := value(args, "--out", "")
	if out == "" {
		return errors.New("--out is required")
	}

	src, err := source(args)
	if err != nil {
		return err
	}

	model := value(args, "--device", "")
	if model != "" {
		err = oddc.FetchModel(src, model, out)
	} else {
		identity := oddc.ReadIdentity(value(args, "--sys", "/sys"))

		model, err = oddc.Fetch(src, identity, out)
		if errors.Is(err, oddc.ErrNoModelMatch) {
			return noMatch(identity)
		}
	}
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", model, src.Revision())
	return nil
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
