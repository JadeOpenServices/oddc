// SPDX-License-Identifier: GPL-3.0-or-later

package catalog_test

import (
	"errors"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func TestClassifyEveryModel(t *testing.T) {
	registry, models := fixture.Catalog(t)

	for _, model := range models {
		path := fixture.FactsFile(t, registry, model)
		if err := catalog.RunClassify(registry, []string{"classify", "--facts", path}); err != nil {
			t.Errorf("%s: %v", model, err)
		}

		sys := fixture.Sysfs(t, registry, model)
		if err := catalog.RunClassify(registry, []string{"classify", "--sys", sys}); err != nil {
			t.Errorf("%s from sysfs: %v", model, err)
		}
	}
}

func TestClassifyUnknownMachineFails(t *testing.T) {
	registry, _ := fixture.Catalog(t)
	sys := fixture.Sysfs(t, registry, "")

	err := catalog.RunClassify(registry, []string{"classify", "--sys", sys})
	if !errors.Is(err, oddc.ErrNoModelMatch) {
		t.Fatalf("err = %v", err)
	}
}
