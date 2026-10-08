// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// upstream is a git repository holding the catalog on a staging branch,
// served as GitHub serves one: partial clones and fetches by commit ID.
func upstream(t *testing.T) (remote, sha string) {
	t.Helper()

	root := fixture.GitCatalog(t)
	git(t, root, "branch", "staging")
	git(t, root, "config", "uploadpack.allowFilter", "true")
	git(t, root, "config", "uploadpack.allowAnySHA1InWant", "true")

	return "file://" + root, git(t, root, "rev-parse", "HEAD")
}

// fetched lists the files whose contents a source's repository holds.
func fetched(t *testing.T, dir, sha string) []string {
	t.Helper()

	held := map[string]bool{}
	objects := git(t, dir, "cat-file", "--batch-all-objects", "--batch-check=%(objecttype) %(objectname)")
	for _, line := range strings.Split(objects, "\n") {
		if kind, name, _ := strings.Cut(line, " "); kind == "blob" {
			held[name] = true
		}
	}

	var files []string
	for _, line := range strings.Split(git(t, dir, "ls-tree", "-r", sha), "\n") {
		meta, file, _ := strings.Cut(line, "\t")
		if fields := strings.Fields(meta); held[fields[2]] {
			files = append(files, file)
		}
	}
	return files
}

func TestFetchEveryModelFromGit(t *testing.T) {
	registry := catalog(t)
	remote, sha := upstream(t)
	catalogFiles := len(strings.Split(git(t, strings.TrimPrefix(remote, "file://"), "ls-files"), "\n"))

	for _, model := range models(t, registry) {
		dir := filepath.Join(t.TempDir(), "git")
		source, err := NewGitSource(dir, remote, "staging")
		if err != nil {
			t.Fatal(err)
		}
		if source.Revision() != sha {
			t.Fatalf("revision %s, want %s", source.Revision(), sha)
		}

		out := filepath.Join(t.TempDir(), "oddc")
		matched, err := Fetch(source, mustIdentity(t, registry, model), out)
		if err != nil || matched != model {
			t.Fatalf("Fetch for %s = %q, %v", model, matched, err)
		}

		expectAnswer(t, registry, model, out, sha)

		// Besides model files, only the closures of models whose own
		// identity does not rule them out may be downloaded.
		identity := mustIdentity(t, registry, model)
		identity.FormFactor = ""
		allowed := map[string]bool{}
		for _, other := range models(t, registry) {
			if plausible(registry.Entities[other].Data, identity) || other == model {
				for _, id := range closureOf(registry, other) {
					allowed["catalog/entities/"+id+".json"] = true
				}
			}
		}

		downloaded := fetched(t, dir, sha)
		for _, file := range downloaded {
			if strings.HasPrefix(file, "evidence/") &&
				!strings.HasPrefix(file, "evidence/"+model+"/") {
				t.Errorf("%s fetch downloaded another model's evidence %s", model, file)
			}
			if strings.HasPrefix(file, "catalog/entities/") &&
				!strings.HasPrefix(file, "catalog/entities/model/") &&
				!allowed[file] {
				t.Errorf("%s fetch downloaded unrelated %s", model, file)
			}
			if !strings.HasPrefix(file, "catalog/entities/") &&
				!strings.HasPrefix(file, "evidence/") {
				t.Errorf("%s fetch downloaded %s", model, file)
			}
		}
		if len(downloaded) == 0 || len(downloaded) >= catalogFiles {
			t.Errorf("%s fetch downloaded %d of %d catalog files", model, len(downloaded), catalogFiles)
		}
	}
}

func TestGitSourcePinsCommitID(t *testing.T) {
	remote, sha := upstream(t)

	source, err := NewGitSource(filepath.Join(t.TempDir(), "git"), remote, sha)
	if err != nil {
		t.Fatal(err)
	}
	if source.Revision() != sha {
		t.Errorf("revision %s, want %s", source.Revision(), sha)
	}
}

func TestGitSourceRejectsUnknownRef(t *testing.T) {
	remote, _ := upstream(t)

	if _, err := NewGitSource(filepath.Join(t.TempDir(), "git"), remote, "no-such-branch"); err == nil {
		t.Error("resolved a ref the remote does not have")
	}
}

func TestDirSourceListsNothingForMissingDir(t *testing.T) {
	files, err := DirSource{Root: "."}.List("evidence/does-not-exist")
	if err != nil || len(files) != 0 {
		t.Errorf("List = %v, %v; want nothing", files, err)
	}
}
