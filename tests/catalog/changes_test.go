// SPDX-License-Identifier: GPL-3.0-or-later

package catalog_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestClass(t *testing.T) {
	for file, want := range map[string]string{
		"evidence/models/framework/x/2026-01-01.json": "evidence",
		"catalog/models/framework/x.json":             "data",
		"schemas/entity.schema.json":                  "schema",
		"flake.nix":                                   "nix",
		"flake.lock":                                  "nix",
		"nixos/module.nix":                            "nix",
		"internal/catalog/changes.go":                 "go",
		"go.mod":                                      "go",
		"README.md":                                   "docs",
		"docs/WORKFLOW.md":                            "docs",
		".github/workflows/ci.yml":                    "other",
		"templates/device/model.json.example":         "other",
	} {
		if got := catalog.Class(file); got != want {
			t.Errorf("%s: class %s, want %s", file, got, want)
		}
	}
}

// TestListChanges commits this repository's own go.mod and README.md
// over a git copy of the catalog, along with a change to existing evidence.
func TestListChanges(t *testing.T) {
	root := fixture.GitCatalog(t)
	evidence := fixture.FirstEvidence(t, root)
	relative, err := filepath.Rel(root, evidence)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range []string{"go.mod", "README.md"} {
		data, err := os.ReadFile(filepath.Join(fixture.Repository, file))
		if err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, filepath.Join(root, file), data)
	}
	if err := os.Remove(evidence); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "next"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	changes, err := catalog.ListChanges(root, "base", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.From) != 40 || len(changes.To) != 40 || changes.From == changes.To {
		t.Errorf("revisions %q and %q are not two commits", changes.From, changes.To)
	}

	want := map[string]catalog.Change{
		"README.md": {Path: "README.md", Status: "A", Class: "docs"},
		"go.mod":    {Path: "go.mod", Status: "A", Class: "go"},
		relative:    {Path: relative, Status: "D", Class: "evidence", AppendOnly: true},
	}
	if len(changes.Changes) != len(want) {
		t.Fatalf("changes %+v, want %+v", changes.Changes, want)
	}
	for _, change := range changes.Changes {
		if change != want[change.Path] {
			t.Errorf("%+v, want %+v", change, want[change.Path])
		}
	}

	if _, err := catalog.ListChanges(root, "no-such-revision", "HEAD"); err == nil {
		t.Error("an unknown revision passed")
	}
}
