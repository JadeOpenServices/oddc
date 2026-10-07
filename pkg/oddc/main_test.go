package oddc

import (
	"os"
	"testing"
)

// The tests read the catalog, evidence and schemas from the repository
// root.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
