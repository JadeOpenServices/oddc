// SPDX-License-Identifier: GPL-3.0-or-later

package system_test

import (
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestDoctorOnEveryModel(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for i, model := range models {
		root := fixture.Deployment(t, registry, model, true)

		if err := system.RunDoctor([]string{
			"doctor", "--root", root, "--sys", fixture.Sysfs(t, registry, model),
		}); err != nil {
			t.Errorf("%s on its own hardware: %v", model, err)
		}

		if err := system.RunDoctor([]string{
			"doctor", "--root", root, "--sys", fixture.Sysfs(t, registry, ""),
		}); err == nil {
			t.Errorf("%s passed on an unknown machine", model)
		}

		if other := models[(i+1)%len(models)]; other != model {
			if err := system.RunDoctor([]string{
				"doctor", "--root", root, "--sys", fixture.Sysfs(t, registry, other),
			}); err == nil {
				t.Errorf("%s passed on %s hardware", model, other)
			}
		}
	}
}

func TestDoctorWithoutDeploymentFails(t *testing.T) {
	if err := system.RunDoctor([]string{"doctor", "--root", t.TempDir()}); err == nil {
		t.Fatal("doctor passed without a deployment")
	}
}

// Doctor tells the user about every component of the deployed model that
// cannot be used on Linux, and still passes.
func TestDoctorNamesUnsupportedComponents(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		root := fixture.Deployment(t, registry, model, false)
		out := fixture.Stdout(t, func() error {
			return system.RunDoctor([]string{"doctor", "--root", root, "--sys", fixture.Sysfs(t, registry, model)})
		})

		for _, part := range oddc.UnsupportedComponents(resolved.Resolved) {
			want := "INFO: " + part.Name + " (" + part.Path + ") cannot be used on Linux: " + part.Reason
			if !strings.Contains(out, want) {
				t.Errorf("%s: doctor output lacks %q:\n%s", model, want, out)
			}
		}
	}
}
