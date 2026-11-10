// SPDX-License-Identifier: GPL-3.0-or-later

package fixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// GitCatalog copies the catalog and evidence into a new git repository
// with one commit, tagged base.
func GitCatalog(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"catalog", "evidence"} {
		err := filepath.WalkDir(filepath.Join(Repository, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}

			relative, err := filepath.Rel(Repository, path)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			Write(t, filepath.Join(root, relative), data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	NoMaintenance(t, root)
	for _, args := range [][]string{
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base"},
		{"tag", "base"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	return root
}

// NoMaintenance keeps git from maintaining the throwaway repository at
// dir. commit, merge and fetch in it, and receive-pack after a push to it,
// otherwise launch a detached `git maintenance run --auto`. git's default
// geometric strategy then repacks the loose objects of a fresh local clone
// once objects/17 holds two of them, writing objects/pack and info/refs
// while the test's TempDir cleanup removes the repository ("directory not
// empty").
func NoMaintenance(t *testing.T, dir string) {
	t.Helper()

	for _, args := range [][]string{
		{"config", "gc.auto", "0"},
		{"config", "maintenance.auto", "false"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// FirstEvidence is the first evidence file below root.
func FirstEvidence(t *testing.T, root string) string {
	t.Helper()

	var found string
	filepath.WalkDir(filepath.Join(root, "evidence"), func(path string, entry os.DirEntry, err error) error {
		if err == nil && found == "" && strings.HasSuffix(path, ".json") {
			found = path
		}
		return err
	})
	if found == "" {
		t.Fatal("catalog has no evidence")
	}

	return found
}
