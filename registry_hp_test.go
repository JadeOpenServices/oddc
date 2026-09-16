package oddc

import (
	"fmt"
	"testing"
)

func TestHPNormalizedGraphPreservesGJAL84Knowledge(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := registry.ResolveEntity(
		"model/hp/zbook-x2-g4",
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"hardware.graphics.integrated.driver": "i915",

		"hardware.graphics.discrete.name": "NVIDIA Quadro M620",

		"hardware.graphics.discrete.deviceId": "10de:13b4",

		"hardware.graphics.discrete.driver": "nvidia",

		"policy.graphics.discrete.driverBranch": "legacy_580",

		"hardware.input.touchscreen.primary.deviceId": "03eb:8abb",

		"hardware.input.touchscreen.primary.classification": "touchscreen",

		"hardware.input.pen.primary.deviceId": "056a:016c",

		"hardware.input.pen.primary.classification": "pen",

		"hardware.input.quickKeys.primary.deviceId": "03f0:0a56",

		"hardware.input.quickKeys.primary.driver": "hid-generic",

		"hardware.input.quickKeys.primary.access.hidraw": "true",

		"hardware.input.quickKeys.primary.access.mappable": "true",

		"hardware.input.quickKeys.primary.controls.buttonCount": "6",

		"hardware.input.quickKeys.primary.controls.presetCount": "3",

		"hardware.input.quickKeys.primary.protocol.reportLength": "6",

		"hardware.input.quickKeys.primary.protocol.reportId": "2",

		"hardware.input.quickKeys.primary.protocol.buttons.button3.raw": "4",

		"hardware.input.quickKeys.primary.protocol.buttons.button3.presetSwitch": "true",

		"hardware.input.quickKeys.primary.protocol.presets.preset3.raw": "4",

		"support.lifecycle.status": "experimental",
	}

	for path, want := range tests {
		got, exists := Lookup(resolved.Resolved, path)
		if !exists {
			t.Fatalf("%s missing", path)
		}

		if fmt.Sprint(got) != want {
			t.Fatalf(
				"%s=%v want=%s",
				path,
				got,
				want,
			)
		}
	}
}
