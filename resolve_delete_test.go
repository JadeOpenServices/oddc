package oddc

import "testing"

func TestHostOverlayCanDeleteCanonicalNode(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	const (
		modelID = "model/framework/laptop-13-amd-ryzen-7040"
		path    = "hardware.security.fingerprint.primary"
	)

	host := Overlay{
		APIVersion:  EntityAPIVersion,
		ID:          "host/test-hardware-removal",
		Kind:        "host",
		TargetModel: modelID,
		Overrides: map[string]any{
			"hardware": map[string]any{
				"security": map[string]any{
					"fingerprint": map[string]any{
						"primary": map[string]any{
							"$delete": true,
						},
					},
				},
			},
		},
	}

	resolved, err := registry.ResolveModel(
		modelID,
		nil,
		[]Overlay{host},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, exists := Lookup(resolved.Resolved, path); exists {
		t.Fatalf("%s survived host deletion", path)
	}

	if _, exists := Lookup(
		resolved.Resolved,
		path+".deviceId",
	); exists {
		t.Fatalf("%s descendants survived host deletion", path)
	}

	if _, exists := resolved.Provenance[path]; exists {
		t.Fatalf("%s retained current provenance after deletion", path)
	}

	history := resolved.History[path]
	if len(history) != 1 {
		t.Fatalf("parent deletion history = %#v, want tombstone only", history)
	}

	last := history[0]
	if last.Source != "host:host/test-hardware-removal" {
		t.Fatalf("deletion source = %q", last.Source)
	}

	marker, ok := last.Value.(map[string]any)
	if !ok || marker["$delete"] != true {
		t.Fatalf("deletion history lost tombstone: %#v", last.Value)
	}

	leafHistory := resolved.History[path+".deviceId"]
	if len(leafHistory) == 0 {
		t.Fatal("canonical leaf history was lost by parent deletion")
	}

	if leafHistory[0].Source == "host:host/test-hardware-removal" {
		t.Fatalf(
			"canonical leaf history was replaced by deletion: %#v",
			leafHistory,
		)
	}
}

func TestOverlayRejectsMalformedDeleteMarker(t *testing.T) {
	tests := []map[string]any{
		{"$delete": false},
		{"$delete": "true"},
		{
			"$delete":  true,
			"deviceId": "27c6:609c",
		},
	}

	for _, marker := range tests {
		err := validateOverrideObject(
			map[string]any{
				"hardware": map[string]any{
					"component": marker,
				},
			},
			"overrides",
		)
		if err == nil {
			t.Fatalf("accepted malformed deletion marker: %#v", marker)
		}
	}
}
