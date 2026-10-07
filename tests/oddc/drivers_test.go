package oddc_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// Every component of a model that the catalog names a driver for is
// reported with that driver once the kernel bound it; nothing else is.
func TestComponentDrivers(t *testing.T) {
	registry := catalog(t)
	bound := 0

	for _, model := range models(t, registry) {
		sys := fixture.Sysfs(t, registry, model)
		fixture.Components(t, sys, registry, model)

		drivers, err := registry.ComponentDrivers(model, ReadDrivers(sys))
		if err != nil {
			t.Fatal(err)
		}

		want := map[string][]string{}
		for _, ref := range collectRefs(registry.Entities[model].Data) {
			data := registry.Entities[ref].Data
			id, _ := data["deviceId"].(string)
			driver, _ := data["driver"].(string)
			if strings.Contains(id, ":") && driver != "" {
				want[ref] = []string{driver}
			}
		}

		bound += len(want)
		if len(drivers) != len(want) {
			t.Errorf("%s: drivers %v, want %v", model, drivers, want)
		}
		for ref, names := range want {
			if !slices.Equal(drivers[ref], names) {
				t.Errorf("%s: %s bound %v, want %v", model, ref, drivers[ref], names)
			}
		}

		if empty, _ := registry.ComponentDrivers(model, ReadDrivers(filepath.Join(sys, "none"))); len(empty) != 0 {
			t.Errorf("%s: drivers %v without a sysfs", model, empty)
		}
	}

	if bound == 0 {
		t.Fatal("no component of any model names a driver")
	}
}
