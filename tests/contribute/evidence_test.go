package contribute_test

import (
	"encoding/json"
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
