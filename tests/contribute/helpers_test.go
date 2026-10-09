// SPDX-License-Identifier: GPL-3.0-or-later

package contribute_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// The GitHub half of contribute (fork, push, pull request) is not tested
// here: it needs the network and a real account, so it is run by hand.

const framework13 = "model/framework/laptop-13-amd-ryzen-7040"

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := cli.Git(dir, args...)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

// catalogUpstream makes a bare repository holding the catalog on staging,
// less the models named in without and their evidence, and a workspace
// cloned from it, as `oddc workspace` does.
func catalogUpstream(t *testing.T, without ...string) (origin, workspace string) {
	t.Helper()

	source := fixture.GitCatalog(t)
	gitOut(t, source, "branch", "-M", contribute.BaseBranch)
	if len(without) > 0 {
		for _, model := range without {
			gitOut(t, source, "rm", "-q", "-r", "--",
				"catalog/entities/"+model+".json", "evidence/"+model)
		}
		gitOut(t, source, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "without")
	}

	origin = filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "clone", "-q", "--bare", source, origin).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	workspace = filepath.Join(t.TempDir(), "workspace")
	if err := contribute.RunWorkspace([]string{"workspace", "--root", workspace, "--from", origin}); err != nil {
		t.Fatal(err)
	}

	return origin, workspace
}

// refs lists the paths of the objects holding a ref, as "path ref".
func refs(object map[string]any, path string, found map[string]string) {
	if ref, ok := object["ref"].(string); ok {
		found[path] = ref
	}

	for key, value := range object {
		if child, ok := value.(map[string]any); ok {
			refs(child, strings.TrimPrefix(path+"."+key, "."), found)
		}
	}
}

func today() string {
	return time.Now().UTC().Format(time.DateOnly)
}

// unusedDate is the latest day up to today on which the catalog at root
// holds no evidence, so a test's record never meets a real one.
func unusedDate(t *testing.T, root string) string {
	t.Helper()

	for day := time.Now().UTC(); ; day = day.AddDate(0, 0, -1) {
		date := day.Format(time.DateOnly)
		found, err := filepath.Glob(filepath.Join(root, "evidence", "*", "*", "*", date+"*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(found) == 0 {
			return date
		}
	}
}
