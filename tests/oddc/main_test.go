// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"os"
	"testing"

	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// The tests read the catalog, evidence and schemas from the repository
// root.
func TestMain(m *testing.M) {
	if err := os.Chdir(fixture.Repository); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
