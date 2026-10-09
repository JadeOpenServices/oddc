// SPDX-License-Identifier: GPL-3.0-or-later

package contribute_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestEvidenceRecordOnlyAdds(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, models := fixture.Catalog(t)
	date := unusedDate(t, workspace)

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
	date := unusedDate(t, workspace)

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

// A record names what was tested: the BIOS, the deployed ODDC revision and
// closure, and the drivers bound to the model's components. It runs on the
// model the catalog has evidence for, with the BIOS of the Framework Laptop
// 13 that recorded it and the catalog's own commit as the revision.
// recordable deploys the model of the first real evidence on a machine
// whose /sys holds its components, and returns a workspace to record in.
func recordable(t *testing.T) (registry *oddc.Registry, model, revision, workspace, sys, deployment string) {
	t.Helper()

	origin, workspace := catalogUpstream(t)
	registry, _ = fixture.Catalog(t)

	data, err := os.ReadFile(fixture.FirstEvidence(t, fixture.Repository))
	if err != nil {
		t.Fatal(err)
	}
	var existing oddc.Evidence
	if err := json.Unmarshal(data, &existing); err != nil {
		t.Fatal(err)
	}
	model = existing.DeviceID

	sys = fixture.Sysfs(t, registry, model)
	fixture.Components(t, sys, registry, model)
	fixture.Write(t, filepath.Join(sys, "class", "dmi", "id", "bios_version"), []byte("03.20\n"))
	revision = gitOut(t, origin, "rev-parse", "HEAD")
	deployment = fixture.Deployment(t, registry, model, false)
	fixture.Write(t, filepath.Join(deployment, "revision"), []byte(revision+"\n"))

	return registry, model, revision, workspace, sys, deployment
}

func TestEvidenceRecordWhatWasTested(t *testing.T) {
	registry, model, revision, workspace, sys, deployment := recordable(t)
	date := unusedDate(t, workspace)
	closure, err := registry.Closure(model)
	if err != nil {
		t.Fatal(err)
	}

	if err := contribute.RunEvidence([]string{
		"evidence", "record", "--root", workspace, "--sys", sys, "--deployment", deployment,
		"--result", "wifi=pass", "--date", date,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(workspace, "evidence", filepath.FromSlash(model), date+".json"))
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
	if record.Closure != closure {
		t.Errorf("closure %q, want %q", record.Closure, closure)
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

// Passing evidence is written only when it proves the model.
func TestEvidenceRecordPassingNeedsProof(t *testing.T) {
	registry, model, _, workspace, sys, deployment := recordable(t)
	date := unusedDate(t, workspace)
	record := []string{
		"evidence", "record", "--root", workspace, "--sys", sys, "--deployment", deployment,
		"--status", "hardware-validated", "--date", date,
	}

	err := contribute.RunEvidence(append(record, "--result", "hardware.network.wifi.primary=pass"))
	if err == nil || !strings.Contains(err.Error(), "does not prove") {
		t.Fatalf("unproven evidence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "evidence", filepath.FromSlash(model), date+".json")); !os.IsNotExist(err) {
		t.Fatalf("unproven evidence was written: %v", err)
	}

	required, err := registry.Requirements(model)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range required {
		if name != "identity" {
			record = append(record, "--result", name+"="+want)
		}
	}
	out := stdout(t, func() error { return contribute.RunEvidence(record) })
	// The status comes from the catalog the record went into, not /etc/oddc.
	if want := model + " in " + workspace + ": verified"; !strings.Contains(out, want) {
		t.Errorf("output %q, want %q", out, want)
	}
	if result := oddc.Validate(workspace); !result.Valid {
		t.Fatal(result.Errors)
	}
}

// A kernel-ranged quirk the deployment did not apply is recorded as such
// and must be proven not-affected.
func TestEvidenceRecordInactiveQuirks(t *testing.T) {
	registry, model, _, workspace, sys, deployment := recordable(t)
	date := unusedDate(t, workspace)
	resolved, err := registry.ResolveEntity(model)
	if err != nil {
		t.Fatal(err)
	}
	ranged := ""
	for key, quirk := range resolved.Resolved["quirks"].(map[string]any) {
		if _, ok := quirk.(map[string]any)["affected"]; ok {
			ranged = key
		}
	}
	if ranged == "" {
		t.Fatalf("%s has no kernel-ranged quirk", model)
	}
	fixture.Write(t, filepath.Join(deployment, "inactive-quirks.json"), []byte(`["`+ranged+`"]`))

	required, err := registry.Requirements(model)
	if err != nil {
		t.Fatal(err)
	}
	record := []string{
		"evidence", "record", "--root", workspace, "--sys", sys, "--deployment", deployment,
		"--status", "hardware-validated", "--date", date,
	}
	for name, want := range required {
		if name != "identity" {
			record = append(record, "--result", name+"="+want)
		}
	}

	err = contribute.RunEvidence(record)
	if want := "result quirks." + ranged + ": pass, want not-affected"; err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("inactive %s recorded as pass: %v, want %q", ranged, err, want)
	}

	if err := contribute.RunEvidence(append(record, "--result", "quirks."+ranged+"=not-affected")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "evidence", filepath.FromSlash(model), date+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var written oddc.Evidence
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(written.InactiveQuirks) != fmt.Sprint([]string{ranged}) {
		t.Errorf("inactive quirks %v, want [%s]", written.InactiveQuirks, ranged)
	}
	if result := oddc.Validate(workspace); !result.Valid {
		t.Fatal(result.Errors)
	}
}
