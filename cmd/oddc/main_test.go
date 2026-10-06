package main

import (
	"os"
	"path/filepath"
	"slices"
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
