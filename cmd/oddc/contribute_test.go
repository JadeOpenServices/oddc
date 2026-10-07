package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// The GitHub half of contribute (fork, push, pull request) is not tested
// here: it needs the network and a real account, so it is run by hand.

const framework13 = "model/framework/laptop-13-amd-ryzen-7040"

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := git(dir, args...)
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

	source := gitCatalog(t)
	gitOut(t, source, "branch", "-M", baseBranch)
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
	if err := runWorkspace([]string{"workspace", "--root", workspace, "--from", origin}); err != nil {
		t.Fatal(err)
	}

	return origin, workspace
}

// modelFacts writes the facts a machine of a catalog model reports.
func modelFacts(t *testing.T, model string) string {
	t.Helper()

	registry, _ := catalog(t)
	facts, err := registry.ModelFacts(model)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "facts.json")
	write(t, file, data)

	return file
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

// Scaffolding the Framework 13 into a catalog that lacks it redrafts it
// from its facts: same identity, and its components where they were.
func TestScaffoldRedraftsCatalogModel(t *testing.T) {
	_, workspace := catalogUpstream(t, framework13)
	facts := modelFacts(t, framework13)
	args := []string{"scaffold", "--root", workspace, "--facts", facts, "--id", framework13}

	if err := runScaffold(args); err != nil {
		t.Fatal(err)
	}

	registry, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}
	full, _ := catalog(t)

	modelFacts, err := full.ModelFacts(framework13)
	if err != nil {
		t.Fatal(err)
	}
	classification, err := registry.Classify(modelFacts)
	if err != nil {
		t.Fatal(err)
	}
	if classification.Model != framework13 {
		t.Fatalf("draft does not classify: %+v", classification)
	}

	drafted, original := map[string]string{}, map[string]string{}
	refs(registry.Entities[framework13].Data, "", drafted)
	refs(full.Entities[framework13].Data, "", original)
	for _, path := range []string{"vendor", "class"} {
		if drafted[path] != original[path] {
			t.Errorf("%s = %q want %q", path, drafted[path], original[path])
		}
	}
	for path, ref := range drafted {
		if original[path] != ref {
			t.Errorf("drafted %s at %s, which the catalog model has as %q", ref, path, original[path])
		}
	}

	if err := runScaffold(args); err == nil {
		t.Fatal("scaffolded a machine that already matches")
	}
}

func TestEvidenceRecordOnlyAdds(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, models := catalog(t)
	date := today()

	for _, model := range models {
		sys := sysfs(t, registry, model)
		args := []string{
			"evidence", "record", "--root", workspace, "--sys", sys,
			"--result", "wifi=pass", "--date", date,
		}
		for range 2 {
			if err := runEvidence(args); err != nil {
				t.Fatalf("%s: %v", model, err)
			}
		}

		dir := filepath.Join(workspace, "evidence", filepath.FromSlash(model))
		for _, name := range []string{date + ".json", date + "-2.json"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}

			var record oddc.Evidence
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			if record.DeviceID != model || record.Results["identity"] != "pass" || record.Results["wifi"] != "pass" {
				t.Errorf("%s/%s: %+v", model, name, record)
			}
		}
	}

	if result := oddc.Validate(workspace); !result.Valid {
		t.Fatal(result.Errors)
	}
}

func TestEvidenceRecordRefusesIdentifyingResult(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, models := catalog(t)
	sys := sysfs(t, registry, models[0])
	date := today()

	err := runEvidence([]string{
		"evidence", "record", "--root", workspace, "--sys", sys,
		"--result", "wifi=works at home", "--date", date,
	})
	if err == nil {
		t.Fatal("recorded a free-text result")
	}

	file := filepath.Join(workspace, "evidence", filepath.FromSlash(models[0]), date+".json")
	if exists(file) {
		t.Fatal("refused record was left behind")
	}
}

// A new model and its evidence become one tree on staging, without
// touching the workspace; once merged, updating the workspace takes them.
func TestContributeNewModel(t *testing.T) {
	origin, workspace := catalogUpstream(t, framework13)
	facts := modelFacts(t, framework13)

	if err := runScaffold([]string{"scaffold", "--root", workspace, "--facts", facts, "--id", framework13}); err != nil {
		t.Fatal(err)
	}
	if err := runEvidence([]string{"evidence", "record", "--root", workspace, "--facts", facts}); err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, workspace, "rev-parse", "HEAD")
	status := gitOut(t, workspace, "status", "--porcelain", "--untracked-files=all")

	p, err := prepareContribution([]string{"contribute", "--root", workspace})
	if err != nil {
		t.Fatal(err)
	}

	if want := "Add " + framework13; p.title != want {
		t.Errorf("title %q want %q", p.title, want)
	}
	sent := gitOut(t, workspace, "diff-tree", "-r", "--name-status", p.base, p.tree)
	want := "A\tcatalog/entities/" + framework13 + ".json\nA\tevidence/" + framework13 + "/" + today() + ".json"
	if sent != want {
		t.Errorf("sends %q want %q", sent, want)
	}

	if gitOut(t, workspace, "rev-parse", "HEAD") != head ||
		gitOut(t, workspace, "status", "--porcelain", "--untracked-files=all") != status {
		t.Error("contribute changed the workspace")
	}

	merged := gitOut(t, workspace, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit-tree", p.tree, "-p", p.base, "-m", p.title)
	gitOut(t, workspace, "push", "-q", origin, merged+":refs/heads/"+baseBranch)
	if err := runWorkspace([]string{"workspace", "--root", workspace}); err != nil {
		t.Fatal(err)
	}
	if status := gitOut(t, workspace, "status", "--porcelain"); status != "" {
		t.Errorf("workspace not clean after update: %q", status)
	}
}

func TestContributeEvidence(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, models := catalog(t)

	if err := runEvidence([]string{
		"evidence", "record", "--root", workspace, "--sys", sysfs(t, registry, models[0]),
	}); err != nil {
		t.Fatal(err)
	}

	p, err := prepareContribution([]string{"contribute", "--root", workspace})
	if err != nil {
		t.Fatal(err)
	}
	if want := "Evidence for " + models[0]; p.title != want {
		t.Errorf("title %q want %q", p.title, want)
	}
}

// Nothing is prepared when a check fails.
func TestContributeRefuses(t *testing.T) {
	cases := map[string]func(t *testing.T, workspace string){
		"nothing changed": func(*testing.T, string) {},
		"file outside the catalog": func(t *testing.T, workspace string) {
			write(t, filepath.Join(workspace, "notes.txt"), []byte("x"))
		},
		"changed evidence": func(t *testing.T, workspace string) {
			path := firstEvidence(t, workspace)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			write(t, path, []byte(strings.Replace(string(data), `"pass"`, `"fail"`, 1)))
		},
		"identifying evidence": func(t *testing.T, workspace string) {
			path := firstEvidence(t, workspace)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			added := filepath.Join(filepath.Dir(path), today()+".json")
			write(t, added, []byte(strings.Replace(string(data), `"os":`, `"hostname": "laptop", "os":`, 1)))
		},
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			_, workspace := catalogUpstream(t)
			change(t, workspace)

			if _, err := prepareContribution([]string{"contribute", "--root", workspace}); err == nil {
				t.Fatal("prepared a contribution")
			}
		})
	}
}
