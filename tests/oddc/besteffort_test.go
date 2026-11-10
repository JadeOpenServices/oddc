// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// A graphics option rests on a verified model: in a copy of the catalog
// where that model changed after its evidence, its GPU gets no option.
func TestGraphicsOptionsNeedAVerifiedModel(t *testing.T) {
	registry := catalog(t)
	checked := 0
	for _, model := range models(t, registry) {
		resolved, err := registry.ResolveEntity(model)
		if err != nil {
			t.Fatal(err)
		}
		gpu, _ := Lookup(resolved.Resolved, "hardware.graphics.discrete.deviceId")
		deviceID, _ := gpu.(string)
		if branch, _ := Lookup(resolved.Resolved, "policy.graphics.discrete.driverBranch"); branch == nil || deviceID == "" {
			continue
		}
		checked++

		with, err := registry.GraphicsOptions([]GPU{{DeviceID: deviceID}})
		if err != nil {
			t.Fatal(err)
		}
		if !optionFrom(with[0], model) {
			t.Fatalf("%s: no option from the verified model: %+v", model, with)
		}

		root := copyCatalog(t)
		editEntity(t, entityPath(t, root, model), func(document map[string]any) {
			data := document["data"].(map[string]any)
			for _, key := range []string{"capabilities", "class"} {
				if _, ok := data[key]; ok {
					delete(data, key)
					return
				}
			}
			t.Fatalf("%s: nothing to change", model)
		})
		unproven, err := LoadRegistry(root)
		if err != nil {
			t.Fatal(err)
		}
		if status, _ := unproven.Verify(model); status.Status != Changed {
			t.Fatalf("%s: %s after the change", model, status.Status)
		}
		without, err := unproven.GraphicsOptions([]GPU{{DeviceID: deviceID}})
		if err != nil {
			t.Fatal(err)
		}
		if optionFrom(without[0], model) {
			t.Errorf("%s: option without evidence: %+v", model, without)
		}
	}
	if checked == 0 {
		t.Fatal("no model sets policy.graphics.discrete.driverBranch")
	}
}

func optionFrom(gpu GPU, model string) bool {
	for _, option := range gpu.Options {
		if option.ProvenOn.Model == model {
			return true
		}
	}
	return false
}

// The report endpoint is empty until the server runs, and https after.
func TestReportEndpoint(t *testing.T) {
	info := ContributeInfo()
	if ReportEndpoint == "" && info.ReportEndpoint != nil {
		t.Fatalf("endpoint %v for an empty ReportEndpoint", *info.ReportEndpoint)
	}
	if ReportEndpoint != "" && !strings.HasPrefix(ReportEndpoint, "https://") {
		t.Fatalf("ReportEndpoint %q is not https", ReportEndpoint)
	}
}
