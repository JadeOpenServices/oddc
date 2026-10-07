package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// The catalog these tests run over: the repository itself.
const repository = "../.."

func catalog(t *testing.T) (*oddc.Registry, []string) {
	t.Helper()

	registry, err := oddc.LoadRegistry(repository)
	if err != nil {
		t.Fatal(err)
	}

	var models []string
	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			models = append(models, id)
		}
	}
	sort.Strings(models)

	if len(models) == 0 {
		t.Fatal("catalog has no models")
	}

	return registry, models
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// deployment lays out a model as the NixOS module deploys it: the catalog,
// its evidence, resolved.json and, when set, a host overlay that keeps the
// model's own canonical values.
func deployment(t *testing.T, registry *oddc.Registry, model string, overlay bool) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"catalog", "evidence"} {
		source, err := filepath.Abs(filepath.Join(repository, dir))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(source, filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}

	resolved, err := registry.ResolveModel(model, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(resolved.Resolved)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "resolved.json"), data)

	if overlay {
		hardware, _ := oddc.Lookup(resolved.Resolved, "hardware")
		data, err := json.Marshal(oddc.Overlay{
			APIVersion:  oddc.EntityAPIVersion,
			ID:          "host/" + model,
			Kind:        "host",
			TargetModel: model,
			Overrides:   map[string]any{"hardware": hardware},
		})
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "host-overlay.json"), data)
	}

	return root
}

// sysfs lays out a machine that reports the identity a model declares.
func sysfs(t *testing.T, registry *oddc.Registry, model string) string {
	t.Helper()

	sys := t.TempDir()
	if model == "" {
		return sys
	}

	identity, err := registry.ModelIdentity(model)
	if err != nil {
		t.Fatal(err)
	}

	for name, content := range identity.SysfsFiles() {
		write(t, filepath.Join(sys, name), []byte(content))
	}

	return sys
}

func TestDefaultsUseDeployedSystem(t *testing.T) {
	registry, models := catalog(t)

	for _, model := range models {
		root := deployment(t, registry, model, true)

		got := withDefaults([]string{"resolve"}, root)
		want := []string{
			"resolve",
			"--root", root,
			"--device", model,
			"--host", filepath.Join(root, "host-overlay.json"),
		}
		if !slices.Equal(got, want) {
			t.Errorf("got %q want %q", got, want)
		}

		if err := run(got); err != nil {
			t.Errorf("%s: %v", model, err)
		}
	}
}

func TestDefaultsWithoutDeploymentUseWorkingDirectory(t *testing.T) {
	got := withDefaults([]string{"list"}, t.TempDir())
	want := []string{"list", "--root", "."}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultsSkipOverlayForOtherDevice(t *testing.T) {
	registry, models := catalog(t)
	if len(models) < 2 {
		t.Skip("catalog has one model")
	}

	for i, model := range models {
		root := deployment(t, registry, model, true)
		other := models[(i+1)%len(models)]

		got := withDefaults(
			[]string{"resolve", "--root", root, "--device", other},
			"/nonexistent",
		)
		if has(got, "--host") {
			t.Errorf("%s overlay applied to %s: %q", model, other, got)
		}
	}
}

func TestDefaultsKeepExplicitFlags(t *testing.T) {
	registry, models := catalog(t)

	for _, model := range models {
		root := deployment(t, registry, model, true)
		host := filepath.Join(root, "host-overlay.json")

		got := withDefaults(
			[]string{"explain", "--host", host, "--path", "model.id"},
			root,
		)
		want := []string{
			"explain", "--host", host, "--path", "model.id",
			"--root", root,
			"--device", model,
		}
		if !slices.Equal(got, want) {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestDetectEveryModel(t *testing.T) {
	registry, models := catalog(t)

	for _, model := range models {
		sys := sysfs(t, registry, model)

		_, got, err := detect([]string{"detect", "--root", repository, "--sys", sys})
		if err != nil || got != model {
			t.Errorf("%s: detected %q, err %v", model, got, err)
		}
	}
}

func TestDetectUnknownMachineSuggestsScaffold(t *testing.T) {
	registry, _ := catalog(t)
	sys := sysfs(t, registry, "")

	_, _, err := detect([]string{"detect", "--root", repository, "--sys", sys})
	if err == nil || !strings.Contains(err.Error(), "oddc scaffold") {
		t.Fatalf("err = %v", err)
	}
}

func TestDoctorOnEveryModel(t *testing.T) {
	registry, models := catalog(t)

	for i, model := range models {
		root := deployment(t, registry, model, true)

		if err := runDoctor([]string{
			"doctor", "--root", root, "--sys", sysfs(t, registry, model),
		}); err != nil {
			t.Errorf("%s on its own hardware: %v", model, err)
		}

		if err := runDoctor([]string{
			"doctor", "--root", root, "--sys", sysfs(t, registry, ""),
		}); err == nil {
			t.Errorf("%s passed on an unknown machine", model)
		}

		if other := models[(i+1)%len(models)]; other != model {
			if err := runDoctor([]string{
				"doctor", "--root", root, "--sys", sysfs(t, registry, other),
			}); err == nil {
				t.Errorf("%s passed on %s hardware", model, other)
			}
		}
	}
}

func TestDoctorWithoutDeploymentFails(t *testing.T) {
	if err := runDoctor([]string{"doctor", "--root", t.TempDir()}); err == nil {
		t.Fatal("doctor passed without a deployment")
	}
}

func TestFetchEveryModel(t *testing.T) {
	registry, models := catalog(t)

	for _, model := range models {
		for _, args := range [][]string{
			{"--sys", sysfs(t, registry, model)},
			{"--device", model},
		} {
			out := filepath.Join(t.TempDir(), "oddc")

			if err := run(append([]string{"fetch", "--root", repository, "--out", out}, args...)); err != nil {
				t.Fatalf("%s %v: %v", model, args, err)
			}

			answer, err := oddc.LoadRegistry(out)
			if err != nil {
				t.Fatal(err)
			}

			got, err := answer.MatchModel(mustIdentity(t, registry, model))
			if err != nil || got != model {
				t.Errorf("%s %v: answer matches %q, %v", model, args, got, err)
			}
			for id, entity := range answer.Entities {
				if entity.Kind == "DeviceModel" && id != model {
					t.Errorf("%s %v: answer carries %s", model, args, id)
				}
			}
		}
	}
}

func TestFetchUnknownMachineSuggestsScaffold(t *testing.T) {
	registry, _ := catalog(t)
	out := filepath.Join(t.TempDir(), "oddc")

	err := run([]string{"fetch", "--root", repository, "--out", out, "--sys", sysfs(t, registry, "")})
	if err == nil || !strings.Contains(err.Error(), "oddc scaffold") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Lstat(out); err == nil {
		t.Errorf("unmatched fetch left %s", out)
	}
}

func mustIdentity(t *testing.T, registry *oddc.Registry, model string) oddc.MachineIdentity {
	t.Helper()

	identity, err := registry.ModelIdentity(model)
	if err != nil {
		t.Fatal(err)
	}

	return identity
}

func TestValidateExitsNonZeroOnFailure(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		if err := runValidate(repository, "", asJSON); err != nil {
			t.Errorf("json=%v: catalog refused: %v", asJSON, err)
		}
		if err := runValidate(t.TempDir(), "", asJSON); err == nil {
			t.Errorf("json=%v: empty root passed", asJSON)
		}
	}
}

func TestClassifyEveryModel(t *testing.T) {
	registry, models := catalog(t)

	for _, model := range models {
		facts, err := registry.ModelFacts(model)
		if err != nil {
			t.Fatal(err)
		}

		data, err := json.Marshal(facts)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "facts.json")
		write(t, path, data)

		if err := runClassify(registry, []string{"classify", "--facts", path}); err != nil {
			t.Errorf("%s: %v", model, err)
		}

		sys := sysfs(t, registry, model)
		if err := runClassify(registry, []string{"classify", "--sys", sys}); err != nil {
			t.Errorf("%s from sysfs: %v", model, err)
		}
	}
}

func TestClassifyUnknownMachineFails(t *testing.T) {
	registry, _ := catalog(t)
	sys := sysfs(t, registry, "")

	err := runClassify(registry, []string{"classify", "--sys", sys})
	if !errors.Is(err, oddc.ErrNoModelMatch) {
		t.Fatalf("err = %v", err)
	}
}

// gitCatalog copies the catalog and evidence into a new git repository
// with one commit, tagged base.
func gitCatalog(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"catalog", "evidence"} {
		err := filepath.WalkDir(filepath.Join(repository, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}

			relative, err := filepath.Rel(repository, path)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			write(t, filepath.Join(root, relative), data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, args := range [][]string{
		{"init", "-q"},
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

func firstEvidence(t *testing.T, root string) string {
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

func TestValidateSinceAllowsAddedEvidence(t *testing.T) {
	root := gitCatalog(t)
	path := firstEvidence(t, root)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record["id"] = "added-record"
	record["observedAt"] = "2099-01-01"
	data, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(filepath.Dir(path), "2099-01-01.json"), data)

	if err := runValidate(root, "base", false); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSinceRefusesChangedEvidence(t *testing.T) {
	for what, change := range map[string]func(path string) error{
		"changed": func(path string) error {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(path, append(data, '\n'), 0o644)
		},
		"removed": os.Remove,
	} {
		root := gitCatalog(t)
		if err := change(firstEvidence(t, root)); err != nil {
			t.Fatal(err)
		}

		err := runValidate(root, "base", false)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: err = %v", what, err)
		}
	}
}
