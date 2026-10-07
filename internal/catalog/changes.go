// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
)

// Change is one path that differs between two revisions.
type Change struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Class  string `json:"class"`
	// AppendOnly is set when existing evidence was changed or removed.
	AppendOnly bool `json:"appendOnlyViolation,omitempty"`
}

// Changes is what changed from one revision to another.
type Changes struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Changes []Change `json:"changes"`
}

// Class is what a path of the repository holds: data, evidence, schema,
// nix, go, docs or other.
func Class(file string) string {
	top, _, _ := strings.Cut(file, "/")
	base := path.Base(file)

	switch {
	case top == "evidence":
		return "evidence"
	case top == "catalog":
		return "data"
	case top == "schemas":
		return "schema"
	case path.Ext(file) == ".nix" || base == "flake.lock":
		return "nix"
	case path.Ext(file) == ".go" || base == "go.mod" || base == "go.sum":
		return "go"
	case path.Ext(file) == ".md" || top == "docs":
		return "docs"
	default:
		return "other"
	}
}

// ListChanges classifies every path that differs between from and to in
// the repository at root.
func ListChanges(root, from, to string) (Changes, error) {
	result := Changes{Changes: []Change{}}

	for _, rev := range []struct {
		name  string
		value *string
	}{{from, &result.From}, {to, &result.To}} {
		commit, err := cli.Git(root, "rev-parse", "--verify", "--end-of-options", rev.name+"^{commit}")
		if err != nil {
			return result, fmt.Errorf("%s is not a revision of %s: %w", rev.name, root, err)
		}
		*rev.value = commit
	}

	out, err := cli.Git(root, "diff", "--name-status", "--no-renames", "-z", result.From, result.To)
	if err != nil {
		return result, err
	}

	fields := cli.NulSeparated(out)
	for i := 0; i+1 < len(fields); i += 2 {
		change := Change{
			Path:   fields[i+1],
			Status: fields[i],
			Class:  Class(fields[i+1]),
		}
		change.AppendOnly = change.Class == "evidence" && change.Status != "A"
		result.Changes = append(result.Changes, change)
	}

	return result, nil
}

// RunChanges prints the classified changes between --from and --to
// (default HEAD) in the repository at --root, as a list or with --json.
func RunChanges(args []string) error {
	from := cli.Value(args, "--from", "")
	if from == "" {
		return fmt.Errorf("--from is required")
	}

	changes, err := ListChanges(
		cli.Value(args, "--root", "."),
		from,
		cli.Value(args, "--to", "HEAD"),
	)
	if err != nil {
		return err
	}

	if cli.Has(args, "--json") {
		data, err := json.MarshalIndent(changes, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return nil
	}

	for _, change := range changes.Changes {
		note := ""
		if change.AppendOnly {
			note = "\tevidence is append-only"
		}
		fmt.Printf("%s\t%s\t%s%s\n", change.Status, change.Class, change.Path, note)
	}

	return nil
}
