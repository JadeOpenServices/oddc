// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// securityEntities are the ids of the catalog's Security entities.
func securityEntities(t *testing.T, registry *Registry) []string {
	t.Helper()

	var ids []string
	for id, entity := range registry.Entities {
		if entity.Kind == "Security" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		t.Fatal("the catalog has no Security entity")
	}
	return ids
}

// Every model that references a finding needs evidence that the finding
// holds there, and the finding's flags reach the model as false.
func TestSecurityFindingsNeedConfirmation(t *testing.T) {
	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}

	found := 0
	for _, id := range modelIDs(registry) {
		resolved, err := registry.ResolveEntity(id)
		if err != nil {
			t.Fatal(err)
		}
		findings := Findings(resolved.Resolved)
		if len(findings) == 0 {
			continue
		}
		required, err := registry.Requirements(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range findings {
			found++
			if !strings.HasPrefix(finding.ID, "security/") || finding.Summary == "" {
				t.Errorf("%s: finding at %s resolved as %+v", id, finding.Path, finding)
			}
			if finding.Active() && required[finding.Path] != "confirmed" {
				t.Errorf("%s: %s requires %q, want confirmed", id, finding.Path, required[finding.Path])
			}
			for flag := range finding.Flags {
				if value, named := SecurityFlags(findings)[flag]; !named || value {
					t.Errorf("%s: flag %s combined to %v (named %v), want false", id, flag, value, named)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no model references a security finding")
	}
}

// A flag is false as soon as one active finding says so; a withdrawn one
// no longer counts, and a flag nobody names stays unknown.
func TestSecurityFlagsFailClosed(t *testing.T) {
	findings := []Finding{
		{Status: "open", Flags: map[string]bool{"a": false}},
		{Status: "open", Flags: map[string]bool{"a": true, "b": true}},
		{Status: "withdrawn", Flags: map[string]bool{"c": false}},
	}
	flags := SecurityFlags(findings)
	if flags["a"] != false || flags["b"] != true {
		t.Errorf("flags %v: want a=false, b=true", flags)
	}
	if _, named := flags["c"]; named {
		t.Errorf("a withdrawn finding set flag c: %v", flags)
	}
	if _, named := flags["unknown"]; named {
		t.Errorf("a flag no finding names is in the map: %v", flags)
	}
}

// Validation refuses each way a Security entity can be broken.
func TestSecurityEntityRules(t *testing.T) {
	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}
	id := securityEntities(t, registry)[0]

	cases := map[string]struct {
		edit func(data map[string]any)
		want string
	}{
		"a flag that raises trust": {func(data map[string]any) {
			data["flags"] = map[string]any{"thunderboltDmaProtected": true}
		}, "only lowers trust"},
		"no summary": {func(data map[string]any) { delete(data, "summary") }, "data.summary is missing"},
		"a long summary": {func(data map[string]any) {
			data["summary"] = strings.Repeat("word ", 60)
		}, "at most"},
		"no attack": {func(data map[string]any) {
			delete(data["details"].(map[string]any), "attack")
		}, "data.details.attack is missing"},
		"unknown severity": {func(data map[string]any) { data["severity"] = "scary" }, "data.severity"},
		"unknown status":   {func(data map[string]any) { data["status"] = "fixed" }, "data.status"},
		"modified before reported": {func(data map[string]any) {
			data["modified"] = "2000-01-01"
		}, "modified not before reported"},
		"a fwupd attribute that is not one": {func(data map[string]any) {
			data["fwupdHsi"] = map[string]any{"guess": "IOMMU is off"}
		}, "not an org.fwupd.hsi. attribute"},
		"fwupd attributes as a list": {func(data map[string]any) {
			data["fwupdHsi"] = map[string]any{}
		}, "names fwupd HSI attributes"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := copyCatalog(t)
			editEntity(t, entityPath(t, root, id), func(document map[string]any) {
				c.edit(document["data"].(map[string]any))
			})
			result := Validate(root)
			if result.Valid || !strings.Contains(strings.Join(result.Errors, "\n"), c.want) {
				t.Errorf("valid %v, errors %v; want one with %q", result.Valid, result.Errors, c.want)
			}
		})
	}
}

// Below security/ only Security entities live, so an id there always
// means a finding.
func TestSecurityPathHoldsOnlyFindings(t *testing.T) {
	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}
	id := securityEntities(t, registry)[0]

	root := copyCatalog(t)
	editEntity(t, entityPath(t, root, id), func(document map[string]any) {
		document["kind"] = "Quirk"
	})
	result := Validate(root)
	if result.Valid || !strings.Contains(strings.Join(result.Errors, "\n"), "only Security entities") {
		t.Errorf("valid %v, errors %v", result.Valid, result.Errors)
	}
}

// A planned entry or tip without a summary is refused.
func TestSupportEntriesNeedSummary(t *testing.T) {
	root := copyCatalog(t)
	editEntity(t, entityPath(t, root, "model/hp/zbook-x2-g4"), func(document map[string]any) {
		support := document["data"].(map[string]any)["support"].(map[string]any)
		support["notes"] = map[string]any{"empty": map[string]any{"item": "ODDC-1"}}
	})
	result := Validate(root)
	if result.Valid || !strings.Contains(strings.Join(result.Errors, "\n"), "support.notes.empty needs a summary") {
		t.Errorf("valid %v, errors %v", result.Valid, result.Errors)
	}
}
