// SPDX-License-Identifier: GPL-3.0-or-later

package contribute_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestEvidenceRecordOnlyAdds(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, models := fixture.Catalog(t)
	date := today()

	for _, model := range models {
		sys := fixture.Sysfs(t, registry, model)
		args := []string{
			"evidence", "record", "--root", workspace, "--sys", sys,
			"--result", "wifi=pass", "--date", date,
		}
		for range 2 {
			if err := contribute.RunEvidence(args); err != nil {
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
	registry, models := fixture.Catalog(t)
	sys := fixture.Sysfs(t, registry, models[0])
	date := today()

	err := contribute.RunEvidence([]string{
		"evidence", "record", "--root", workspace, "--sys", sys,
		"--result", "wifi=works at home", "--date", date,
	})
	if err == nil {
		t.Fatal("recorded a free-text result")
	}

	file := filepath.Join(workspace, "evidence", filepath.FromSlash(models[0]), date+".json")
	if cli.Exists(file) {
		t.Fatal("refused record was left behind")
	}
}

// A record names what was tested: the BIOS, the deployed ODDC revision
// and the drivers bound to the model's components. It runs on the model
// the catalog has evidence for, with the BIOS of the Framework Laptop 13
// that recorded it and the catalog's own commit as the revision.
func TestEvidenceRecordWhatWasTested(t *testing.T) {
	origin, workspace := catalogUpstream(t)
	registry, _ := fixture.Catalog(t)

	data, err := os.ReadFile(fixture.FirstEvidence(t, fixture.Repository))
	if err != nil {
		t.Fatal(err)
	}
	var existing oddc.Evidence
	if err := json.Unmarshal(data, &existing); err != nil {
		t.Fatal(err)
	}
	model := existing.DeviceID

	sys := fixture.Sysfs(t, registry, model)
	fixture.Components(t, sys, registry, model)
	fixture.Write(t, filepath.Join(sys, "class", "dmi", "id", "bios_version"), []byte("03.20\n"))
	revision := gitOut(t, origin, "rev-parse", "HEAD")

	if err := contribute.RunEvidence([]string{
		"evidence", "record", "--root", workspace, "--sys", sys,
		"--revision", revision, "--result", "wifi=pass", "--date", today(),
	}); err != nil {
		t.Fatal(err)
	}

	data, err = os.ReadFile(filepath.Join(workspace, "evidence", filepath.FromSlash(model), today()+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record oddc.Evidence
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}

	if record.Environment["bios"] != "03.20" || record.Environment["oddc"] != revision {
		t.Errorf("environment %v, want bios 03.20 and oddc %s", record.Environment, revision)
	}
	want, err := registry.ComponentDrivers(model, oddc.ReadDrivers(sys))
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 || fmt.Sprint(record.Drivers) != fmt.Sprint(want) {
		t.Errorf("drivers %v, want %v", record.Drivers, want)
	}

	if result := oddc.Validate(workspace); !result.Valid {
		t.Fatal(result.Errors)
	}
}
