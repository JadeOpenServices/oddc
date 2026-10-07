package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
)

// Stages a system flake can follow. main holds releases; staging holds
// what was merged since, contributions included, before it is released.
var Stages = map[string]string{
	"main":    cli.Upstream,
	"staging": cli.Upstream + "/staging",
}

// LockedInput is the oddc input as a flake.lock records it.
type LockedInput struct {
	Locked struct {
		Rev string `json:"rev"`
	} `json:"locked"`
	Original struct {
		Type  string `json:"type"`
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
		Ref   string `json:"ref"`
		Rev   string `json:"rev"`
	} `json:"original"`
}

// Stage names what the input follows: a stage, a pinned commit, or
// another branch of ODDC on GitHub; "" when it is not ODDC on GitHub.
func (input LockedInput) Stage() string {
	original := input.Original
	if original.Type != "github" ||
		!strings.EqualFold(original.Owner, "JadeOpenServices") ||
		!strings.EqualFold(original.Repo, "oddc") {
		return ""
	}

	switch {
	case original.Rev != "":
		return "commit " + short(original.Rev)
	case original.Ref == "":
		return "main"
	default:
		return original.Ref
	}
}

func short(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}

	return rev
}

// LockedOddc reads the oddc input from a flake's lock file.
func LockedOddc(flake string) (LockedInput, error) {
	var input LockedInput

	lock, err := os.ReadFile(filepath.Join(flake, "flake.lock"))
	if err != nil {
		return input, fmt.Errorf("%s is not a locked flake: %w", flake, err)
	}

	var parsed struct {
		Root  string                     `json:"root"`
		Nodes map[string]json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(lock, &parsed); err != nil {
		return input, err
	}

	var root struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(parsed.Nodes[parsed.Root], &root); err != nil {
		return input, err
	}

	var node string
	if json.Unmarshal(root.Inputs["oddc"], &node) != nil {
		return input, fmt.Errorf("%s has no oddc input", flake)
	}

	return input, json.Unmarshal(parsed.Nodes[node], &input)
}

// oddcURL matches the oddc input's URL in a flake.nix, whatever it follows.
var oddcURL = regexp.MustCompile(`"github:(?i:JadeOpenServices/oddc)(?:/[^"?]*)?(?:\?[^"]*)?"`)

// SetStage points the oddc input in flake.nix at a stage.
func SetStage(flake, url string) error {
	path := filepath.Join(flake, "flake.nix")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if found := oddcURL.FindAll(data, -1); len(found) != 1 {
		return fmt.Errorf(
			"%s has %d ODDC GitHub URLs, not one; set inputs.oddc.url = %q yourself",
			path, len(found), url,
		)
	}

	return os.WriteFile(path, oddcURL.ReplaceAll(data, []byte(`"`+url+`"`)), 0o644)
}
