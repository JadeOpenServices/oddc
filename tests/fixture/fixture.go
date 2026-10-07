// SPDX-License-Identifier: GPL-3.0-or-later

// Package fixture lays out what the tests run over, all from the
// repository's own catalog and evidence: deployments, machines that report
// a model's identity, and git checkouts of the catalog.
package fixture

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Repository is the root of this repository, found from the working
// directory up, as the tests run in their own folders below it.
var Repository = repository()

func repository() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			panic("no go.mod above the working directory")
		}
		dir = parent
	}
}

// Catalog loads the repository's catalog and its models, sorted.
func Catalog(t *testing.T) (*oddc.Registry, []string) {
	t.Helper()

	registry, err := oddc.LoadRegistry(Repository)
	if err != nil {
		t.Fatal(err)
	}

	var models []string
	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			models = append(models, id)
		}
	}
	sort.Strings(models)

	if len(models) == 0 {
		t.Fatal("catalog has no models")
	}

	return registry, models
}

// Write writes a file, making its directory.
func Write(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Sysfs lays out a machine that reports the identity a model declares;
// for model "" a machine that reports nothing.
func Sysfs(t *testing.T, registry *oddc.Registry, model string) string {
	t.Helper()

	sys := t.TempDir()
	if model == "" {
		return sys
	}

	for name, content := range Identity(t, registry, model).SysfsFiles() {
		Write(t, filepath.Join(sys, name), []byte(content))
	}

	return sys
}

// Identity is the identity a model declares.
func Identity(t *testing.T, registry *oddc.Registry, model string) oddc.MachineIdentity {
	t.Helper()

	identity, err := registry.ModelIdentity(model)
	if err != nil {
		t.Fatal(err)
	}

	return identity
}
