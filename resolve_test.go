package oddc

import (
	"fmt"
	"testing"
)

func TestFrameworkModelCanonicalProvenance(
	t *testing.T,
) {
	registry, err := LoadRegistry(
		repositoryODDCRoot(t),
	)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := registry.ResolveModel(
		"model/framework/laptop-13-amd-ryzen-7040",
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"hardware.processor.primary.name": "catalog:processor/amd/ryzen-7-7840u",

		"hardware.graphics.integrated.driver": "catalog:graphics/amd/radeon-780m",

		"hardware.network.wifi.primary.driver": "catalog:wifi/realtek/rtl8852be",

		"vendor.policy.secureBoot.setupModeStrategy": "catalog:vendor/framework",

		"policy.thermal.fanControl.policy.thermalEnterC": "catalog:model/framework/laptop-13-amd-ryzen-7040",
	}

	for path, source := range tests {
		got, exists := resolved.Provenance[path]
		if !exists {
			t.Fatalf(
				"%s has no provenance",
				path,
			)
		}

		if got.Source != source {
			t.Fatalf(
				"%s source=%q want=%q",
				path,
				got.Source,
				source,
			)
		}
	}
}

func TestModelOverridePrecedenceAndHistory(
	t *testing.T,
) {
	registry, err := LoadRegistry(
		repositoryODDCRoot(t),
	)
	if err != nil {
		t.Fatal(err)
	}

	const modelID = "model/framework/laptop-13-amd-ryzen-7040"

	project := Overlay{
		APIVersion:  EntityAPIVersion,
		ID:          "project/test",
		Kind:        "project",
		TargetModel: modelID,
		Overrides: map[string]any{
			"policy": map[string]any{
				"thermal": map[string]any{
					"fanControl": map[string]any{
						"policy": map[string]any{
							"thermalEnterC": 85,
						},
					},
				},
			},
		},
	}

	host := Overlay{
		APIVersion:  EntityAPIVersion,
		ID:          "host/test",
		Kind:        "host",
		TargetModel: modelID,
		Overrides: map[string]any{
			"policy": map[string]any{
				"thermal": map[string]any{
					"fanControl": map[string]any{
						"policy": map[string]any{
							"thermalEnterC": 87,
						},
					},
				},
			},
		},
	}

	resolved, err := registry.ResolveModel(
		modelID,
		[]Overlay{project},
		[]Overlay{host},
	)
	if err != nil {
		t.Fatal(err)
	}

	const path = "policy.thermal.fanControl.policy.thermalEnterC"

	got, exists := Lookup(
		resolved.Resolved,
		path,
	)
	if !exists {
		t.Fatalf("%s missing", path)
	}

	if fmt.Sprint(got) != "87" {
		t.Fatalf(
			"%s=%v want=87",
			path,
			got,
		)
	}

	if source := resolved.Provenance[path].Source; source != "host:host/test" {
		t.Fatalf(
			"%s source=%q",
			path,
			source,
		)
	}

	history := resolved.History[path]
	if len(history) != 3 {
		t.Fatalf(
			"%s history=%d want=3",
			path,
			len(history),
		)
	}

	if history[0].Source !=
		"catalog:model/framework/laptop-13-amd-ryzen-7040" {
		t.Fatalf(
			"unexpected canonical source: %q",
			history[0].Source,
		)
	}

	if history[1].Source != "project:project/test" {
		t.Fatalf(
			"unexpected project source: %q",
			history[1].Source,
		)
	}

	if history[2].Source != "host:host/test" {
		t.Fatalf(
			"unexpected host source: %q",
			history[2].Source,
		)
	}
}

func TestHPQuickKeysCanonicalOwner(
	t *testing.T,
) {
	registry, err := LoadRegistry(
		repositoryODDCRoot(t),
	)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := registry.ResolveModel(
		"model/hp/zbook-x2-g4",
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	const path = "hardware.input.quickKeys.primary.protocol.reportLength"

	got := resolved.Provenance[path]

	if got.Source !=
		"catalog:quickkeys/hp/03f0-0a56" {
		t.Fatalf(
			"%s source=%q",
			path,
			got.Source,
		)
	}
}
