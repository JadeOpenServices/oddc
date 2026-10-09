// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// SystemRoot is where the NixOS module deploys the selected model.
const SystemRoot = "/etc/oddc"

// DeployedModel returns the model a deployed root was built for, or "".
func DeployedModel(root string) string {
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

const Upstream = "github:JadeOpenServices/oddc"

// WorkspaceDir is the default local catalog checkout.
func WorkspaceDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "oddc")
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "oddc")
}

// Source finds the catalog: GitHub at --rev or the --channel branch when
// either is given, else an explicit --root, the local workspace, or GitHub
// at main. Reading from GitHub downloads only the files a command needs,
// into scratch.
func Source(args []string, scratch string) (oddc.Source, error) {
	remote := Has(args, "--rev") || Has(args, "--channel")
	if root := Value(args, "--root", ""); root != "" {
		if remote {
			return nil, errors.New("--root reads a local catalog, --rev and --channel read GitHub: use one or the other")
		}
		return oddc.DirSource{Root: root}, nil
	}

	if dir := WorkspaceDir(); !remote && Exists(filepath.Join(dir, "catalog")) {
		return oddc.DirSource{Root: dir}, nil
	}

	return oddc.NewGitSource(
		filepath.Join(scratch, "git"),
		oddc.Remote,
		Value(args, "--rev", Value(args, "--channel", "main")),
	)
}

// ReadFacts reads facts from --facts FILE, else the sysfs below --sys.
func ReadFacts(args []string) (oddc.Facts, error) {
	path := Value(args, "--facts", "")
	if path == "" {
		return oddc.ReadFacts(Value(args, "--sys", "/sys")), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return oddc.Facts{}, err
	}

	var facts oddc.Facts
	if err := json.Unmarshal(data, &facts); err != nil {
		return oddc.Facts{}, fmt.Errorf("%s: %w", path, err)
	}

	return facts, nil
}
