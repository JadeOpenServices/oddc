// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Every model declares an identity, and exactly that model matches it.
func TestCatalogEveryModelMatchesOnlyItself(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		identity, err := registry.ModelIdentity(id)
		if err != nil {
			t.Fatal(err)
		}

		if identity.SysVendor == "" && identity.ProductName == "" && identity.BoardName == "" {
			t.Errorf("%s declares no DMI identity", id)
			continue
		}

		matched, err := registry.MatchModel(identity)
		if err != nil || matched != id {
			t.Errorf("%s: own identity matched %q, err %v", id, matched, err)
		}
	}
}

func TestCatalogEmptyIdentityMatchesNothing(t *testing.T) {
	_, err := catalog(t).MatchModel(MachineIdentity{})
	if !errors.Is(err, ErrNoModelMatch) {
		t.Fatalf("err = %v", err)
	}
}

// A machine reporting a model's identity through sysfs is read back as
// that model.
func TestCatalogReadIdentityFromSysfs(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		identity, err := registry.ModelIdentity(id)
		if err != nil {
			t.Fatal(err)
		}

		sys := t.TempDir()
		for name, content := range identity.SysfsFiles() {
			path := filepath.Join(sys, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		if got := ReadIdentity(sys); got != identity {
			t.Errorf("%s: read %+v want %+v", id, got, identity)
		}
	}
}

// Evidence that passed identity belongs to a model that declares one.
func TestCatalogEvidenceIdentityPass(t *testing.T) {
	registry := catalog(t)

	err := filepath.WalkDir("evidence", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var evidence Evidence
		if err := json.Unmarshal(data, &evidence); err != nil {
			return err
		}

		if evidence.Results["identity"] != "pass" {
			return nil
		}

		identity, err := registry.ModelIdentity(evidence.DeviceID)
		if err != nil {
			return err
		}

		if identity.SysVendor == "" && identity.ProductName == "" {
			t.Errorf("%s passed identity, but %s declares none", path, evidence.DeviceID)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
