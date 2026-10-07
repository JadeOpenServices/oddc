package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

// Source finds the catalog: an explicit --root, the local workspace,
// else GitHub at --rev or the --channel branch. Reading from GitHub
// downloads only the files a command needs.
func Source(args []string) (oddc.Source, error) {
	if root := Value(args, "--root", ""); root != "" {
		return oddc.DirSource{Root: root}, nil
	}

	if dir := WorkspaceDir(); Exists(filepath.Join(dir, "catalog")) {
		return oddc.DirSource{Root: dir}, nil
	}

	return oddc.NewGitHubSource(
		&http.Client{Timeout: time.Minute},
		oddc.GitHubAPI,
		oddc.GitHubRaw,
		oddc.Repository,
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
