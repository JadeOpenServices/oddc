// SPDX-License-Identifier: GPL-3.0-or-later

package fixture

import (
	"io"
	"os"
	"testing"
)

// Stdout runs f and returns what it printed; f must not fail.
func Stdout(t *testing.T, f func() error) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = write
	err = f()
	os.Stdout = saved
	write.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
