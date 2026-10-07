// SPDX-License-Identifier: GPL-3.0-or-later

package system_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestDetectEveryModel(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		sys := fixture.Sysfs(t, registry, model)

		_, got, err := system.Detect([]string{"detect", "--root", fixture.Repository, "--sys", sys})
		if err != nil || got != model {
			t.Errorf("%s: detected %q, err %v", model, got, err)
		}
	}
}

func TestDetectUnknownMachineSuggestsScaffold(t *testing.T) {
	registry, _ := fixture.Catalog(t)
	sys := fixture.Sysfs(t, registry, "")

	_, _, err := system.Detect([]string{"detect", "--root", fixture.Repository, "--sys", sys})
	if err == nil || !strings.Contains(err.Error(), "oddc scaffold") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchEveryModel(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		for _, args := range [][]string{
			{"--sys", fixture.Sysfs(t, registry, model)},
			{"--device", model},
		} {
			out := filepath.Join(t.TempDir(), "oddc")

			if err := system.RunFetch(append([]string{"fetch", "--root", fixture.Repository, "--out", out}, args...)); err != nil {
				t.Fatalf("%s %v: %v", model, args, err)
			}

			answer, err := oddc.LoadRegistry(out)
			if err != nil {
				t.Fatal(err)
			}

			got, err := answer.MatchModel(fixture.Identity(t, registry, model))
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
	registry, _ := fixture.Catalog(t)
	out := filepath.Join(t.TempDir(), "oddc")

	err := system.RunFetch([]string{"fetch", "--root", fixture.Repository, "--out", out, "--sys", fixture.Sysfs(t, registry, "")})
	if err == nil || !strings.Contains(err.Error(), "oddc scaffold") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Lstat(out); err == nil {
		t.Errorf("unmatched fetch left %s", out)
	}
}
