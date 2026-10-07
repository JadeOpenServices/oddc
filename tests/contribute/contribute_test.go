package contribute_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// A new model and its evidence become one tree on staging, without
// touching the workspace; once merged, updating the workspace takes them.
func TestContributeNewModel(t *testing.T) {
	origin, workspace := catalogUpstream(t, framework13)
	registry, _ := fixture.Catalog(t)
	facts := fixture.FactsFile(t, registry, framework13)

	if err := contribute.RunScaffold([]string{"scaffold", "--root", workspace, "--facts", facts, "--id", framework13}); err != nil {
		t.Fatal(err)
	}
	if err := contribute.RunEvidence([]string{"evidence", "record", "--root", workspace, "--facts", facts}); err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, workspace, "rev-parse", "HEAD")
	status := gitOut(t, workspace, "status", "--porcelain", "--untracked-files=all")

	p, err := contribute.Prepare([]string{"contribute", "--root", workspace})
	if err != nil {
		t.Fatal(err)
	}

	if want := "Add " + framework13; p.Title != want {
		t.Errorf("title %q want %q", p.Title, want)
	}
	sent := gitOut(t, workspace, "diff-tree", "-r", "--name-status", p.Base, p.Tree)
	want := "A\tcatalog/entities/" + framework13 + ".json\nA\tevidence/" + framework13 + "/" + today() + ".json"
	if sent != want {
		t.Errorf("sends %q want %q", sent, want)
	}

	if gitOut(t, workspace, "rev-parse", "HEAD") != head ||
		gitOut(t, workspace, "status", "--porcelain", "--untracked-files=all") != status {
		t.Error("contribute changed the workspace")
	}

	merged := gitOut(t, workspace, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit-tree", p.Tree, "-p", p.Base, "-m", p.Title)
	gitOut(t, workspace, "push", "-q", origin, merged+":refs/heads/"+contribute.BaseBranch)
	if err := contribute.RunWorkspace([]string{"workspace", "--root", workspace}); err != nil {
		t.Fatal(err)
	}
	if status := gitOut(t, workspace, "status", "--porcelain"); status != "" {
		t.Errorf("workspace not clean after update: %q", status)
	}
}

func TestContributeEvidence(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, models := fixture.Catalog(t)

	if err := contribute.RunEvidence([]string{
		"evidence", "record", "--root", workspace, "--sys", fixture.Sysfs(t, registry, models[0]),
	}); err != nil {
		t.Fatal(err)
	}

	p, err := contribute.Prepare([]string{"contribute", "--root", workspace})
	if err != nil {
		t.Fatal(err)
	}
	if want := "Evidence for " + models[0]; p.Title != want {
		t.Errorf("title %q want %q", p.Title, want)
	}
}

// Nothing is prepared when a check fails.
func TestContributeRefuses(t *testing.T) {
	cases := map[string]func(t *testing.T, workspace string){
		"nothing changed": func(*testing.T, string) {},
		"file outside the catalog": func(t *testing.T, workspace string) {
			fixture.Write(t, filepath.Join(workspace, "notes.txt"), []byte("x"))
		},
		"changed evidence": func(t *testing.T, workspace string) {
			path := fixture.FirstEvidence(t, workspace)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, path, []byte(strings.Replace(string(data), `"pass"`, `"fail"`, 1)))
		},
		"identifying evidence": func(t *testing.T, workspace string) {
			path := fixture.FirstEvidence(t, workspace)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			added := filepath.Join(filepath.Dir(path), today()+".json")
			fixture.Write(t, added, []byte(strings.Replace(string(data), `"os":`, `"hostname": "laptop", "os":`, 1)))
		},
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			_, workspace := catalogUpstream(t)
			change(t, workspace)

			if _, err := contribute.Prepare([]string{"contribute", "--root", workspace}); err == nil {
				t.Fatal("prepared a contribution")
			}
		})
	}
}
