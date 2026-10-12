// SPDX-License-Identifier: GPL-3.0-or-later

package contribute_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// quirkAndModel picks a real quirk and a model that references it
// (referencing true) or does not.
func quirkAndModel(t *testing.T, registry *oddc.Registry, referencing bool) (string, string) {
	t.Helper()

	ids := make([]string, 0, len(registry.Entities))
	for id := range registry.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, quirk := range ids {
		if registry.Entities[quirk].Kind != "Quirk" {
			continue
		}
		for _, model := range ids {
			if registry.Entities[model].Kind != "DeviceModel" {
				continue
			}
			if referencesQuirk(registry.Entities[model].Data, quirk) == referencing {
				return quirk, model
			}
		}
	}
	t.Skip("the catalog has no such quirk and model")
	return "", ""
}

func referencesQuirk(model map[string]any, quirk string) bool {
	quirks, _ := model["quirks"].(map[string]any)
	for _, value := range quirks {
		if entry, _ := value.(map[string]any); entry["ref"] == quirk {
			return true
		}
	}
	return false
}

func editFile(t *testing.T, path string, edit func(map[string]any)) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	edit(document)
	if data, err = json.Marshal(document); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestQuirkEnableAdoptsSharedQuirk(t *testing.T) {
	_, workspace := catalogUpstream(t)
	before, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}
	quirk, model := quirkAndModel(t, before, false)
	args := []string{"quirk", "enable", quirk, "--device", model, "--root", workspace}

	out := stdout(t, func() error { return contribute.RunQuirk(args) })
	if !strings.Contains(out, "oddc evidence record") {
		t.Errorf("no evidence step in %q", out)
	}

	after, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !referencesQuirk(after.Entities[model].Data, quirk) {
		t.Fatalf("%s does not reference %s", model, quirk)
	}
	if result := oddc.Validate(workspace); !result.Valid {
		t.Fatalf("catalog invalid after enable: %v", result.Errors)
	}
	if err := contribute.RunQuirk(args); err == nil {
		t.Error("enabled the same quirk twice")
	}
}

func TestQuirkEnableChecksRequirements(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}
	quirk, model := quirkAndModel(t, registry, false)
	editFile(t, oddc.EntityPath(filepath.Join(workspace, "catalog", "entities"), quirk), func(document map[string]any) {
		document["data"].(map[string]any)["requires"] = map[string]any{"hardware.noSuchPart": true}
	})
	modelPath := oddc.EntityPath(filepath.Join(workspace, "catalog", "entities"), model)
	before, _ := os.ReadFile(modelPath)

	err = contribute.RunQuirk([]string{"quirk", "enable", quirk, "--device", model, "--root", workspace})
	if err == nil || !strings.Contains(err.Error(), "hardware.noSuchPart") {
		t.Fatalf("enable with a missing requirement: %v", err)
	}
	if after, _ := os.ReadFile(modelPath); string(after) != string(before) {
		t.Error("the model changed although the quirk does not fit")
	}
}

func TestQuirkShareMergesCopies(t *testing.T) {
	_, workspace := catalogUpstream(t)
	registry, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}
	quirk, model := quirkAndModel(t, registry, true)
	entities := filepath.Join(workspace, "catalog", "entities")

	copyID := quirk + "-copy"
	data, err := os.ReadFile(oddc.EntityPath(entities, quirk))
	if err != nil {
		t.Fatal(err)
	}
	copyPath := oddc.EntityPath(entities, copyID)
	if err := os.WriteFile(copyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	editFile(t, copyPath, func(document map[string]any) {
		document["metadata"].(map[string]any)["id"] = copyID
	})
	editFile(t, oddc.EntityPath(entities, model), func(document map[string]any) {
		for _, value := range document["data"].(map[string]any)["quirks"].(map[string]any) {
			if entry := value.(map[string]any); entry["ref"] == quirk {
				entry["ref"] = copyID
			}
		}
	})

	out := stdout(t, func() error { return contribute.RunQuirk([]string{"quirk", "share", "--root", workspace}) })
	if !strings.Contains(out, model) {
		t.Errorf("share did not name %s for new evidence: %q", model, out)
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Error("the copy is still there")
	}
	after, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !referencesQuirk(after.Entities[model].Data, quirk) {
		t.Errorf("%s does not reference %s after share", model, quirk)
	}
	if result := oddc.Validate(workspace); !result.Valid {
		t.Fatalf("catalog invalid after share: %v", result.Errors)
	}
}
