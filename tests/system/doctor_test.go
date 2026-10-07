package system_test

import (
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
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
