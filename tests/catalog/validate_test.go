package catalog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestValidateExitsNonZeroOnFailure(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		if err := catalog.RunValidate(fixture.Repository, "", asJSON); err != nil {
			t.Errorf("json=%v: catalog refused: %v", asJSON, err)
		}
		if err := catalog.RunValidate(t.TempDir(), "", asJSON); err == nil {
			t.Errorf("json=%v: empty root passed", asJSON)
		}
	}
}

func TestValidateSinceAllowsAddedEvidence(t *testing.T) {
	root := fixture.GitCatalog(t)
	path := fixture.FirstEvidence(t, root)

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
	fixture.Write(t, filepath.Join(filepath.Dir(path), "2099-01-01.json"), data)

	if err := catalog.RunValidate(root, "base", false); err != nil {
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
		root := fixture.GitCatalog(t)
		if err := change(fixture.FirstEvidence(t, root)); err != nil {
			t.Fatal(err)
		}

		err := catalog.RunValidate(root, "base", false)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: err = %v", what, err)
		}
	}
}
