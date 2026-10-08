// SPDX-License-Identifier: GPL-3.0-or-later

package catalog_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestDefaultsUseDeployedSystem(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		root := fixture.Deployment(t, registry, model, true)

		got := catalog.WithDefaults([]string{"resolve"}, root)
		want := []string{
			"resolve",
			"--root", root,
			"--device", model,
			"--host", filepath.Join(root, "host-overlay.json"),
		}
		if !slices.Equal(got, want) {
			t.Errorf("got %q want %q", got, want)
		}

		if err := catalog.Run(got); err != nil {
			t.Errorf("%s: %v", model, err)
		}
	}
}

func TestDefaultsWithoutDeploymentUseWorkingDirectory(t *testing.T) {
	got := catalog.WithDefaults([]string{"list"}, t.TempDir())
	want := []string{"list", "--root", "."}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultsSkipOverlayForOtherDevice(t *testing.T) {
	registry, models := fixture.Catalog(t)
	if len(models) < 2 {
		t.Skip("catalog has one model")
	}

	for i, model := range models {
		root := fixture.Deployment(t, registry, model, true)
		other := models[(i+1)%len(models)]

		got := catalog.WithDefaults(
			[]string{"resolve", "--root", root, "--device", other},
			"/nonexistent",
		)
		if cli.Has(got, "--host") {
			t.Errorf("%s overlay applied to %s: %q", model, other, got)
		}
	}
}

func TestDefaultsKeepExplicitFlags(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		root := fixture.Deployment(t, registry, model, true)
		host := filepath.Join(root, "host-overlay.json")

		got := catalog.WithDefaults(
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
