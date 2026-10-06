package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func deployment(t *testing.T, overlay bool) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "resolved.json"),
		[]byte(`{"model":{"id":"model/test/a"}}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if overlay {
		if err := os.WriteFile(
			filepath.Join(root, "host-overlay.json"),
			[]byte(`{}`),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func TestDefaultsUseDeployedSystem(t *testing.T) {
	root := deployment(t, true)

	got := withDefaults([]string{"resolve"}, root)
	want := []string{
		"resolve",
		"--root", root,
		"--device", "model/test/a",
		"--host", filepath.Join(root, "host-overlay.json"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
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
	root := deployment(t, true)

	got := withDefaults(
		[]string{"resolve", "--root", root, "--device", "model/test/b"},
		"/nonexistent",
	)
	if has(got, "--host") {
		t.Fatalf("overlay applied to another device: %q", got)
	}
}

func TestDefaultsKeepExplicitFlags(t *testing.T) {
	root := deployment(t, true)

	got := withDefaults(
		[]string{"explain", "--host", "x.json", "--path", "a"},
		root,
	)
	want := []string{
		"explain", "--host", "x.json", "--path", "a",
		"--root", root,
		"--device", "model/test/a",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func fakeSys(t *testing.T, files map[string]string) string {
	t.Helper()

	sys := t.TempDir()
	for name, content := range files {
		path := filepath.Join(sys, "class", "dmi", "id", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return sys
}

func TestDetectMatchesCatalogModel(t *testing.T) {
	sys := fakeSys(t, map[string]string{
		"sys_vendor":   "HP",
		"product_name": "HP ZBook x2 G4",
		"board_name":   "824C",
	})

	_, model, err := detect([]string{"detect", "--root", "../..", "--sys", sys})
	if err != nil || model != "model/hp/zbook-x2-g4" {
		t.Fatalf("model = %q, err = %v", model, err)
	}
}

func TestDetectUnknownMachineSuggestsScaffold(t *testing.T) {
	sys := fakeSys(t, map[string]string{"sys_vendor": "Nobody"})

	_, _, err := detect([]string{"detect", "--root", "../..", "--sys", sys})
	if err == nil || !strings.Contains(err.Error(), "oddc scaffold") {
		t.Fatalf("err = %v", err)
	}
}

func TestDoctorWithoutDeploymentFails(t *testing.T) {
	if err := runDoctor([]string{"doctor", "--root", t.TempDir()}); err == nil {
		t.Fatal("doctor passed without a deployment")
	}
}
