// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// match finds the machine's model. A model owns its DMI identity, so its
// own data rules it out before any of its references are fetched; only the
// remaining candidates are fetched and matched in full.
func (f *fetcher) match(identity MachineIdentity) (string, error) {
	files, err := f.source.List("catalog/entities/model")
	if err != nil {
		return "", err
	}

	unknownFormFactor := identity
	unknownFormFactor.FormFactor = ""

	var candidates []string

	for _, file := range files {
		if path.Ext(file) != ".json" {
			continue
		}

		id := strings.TrimSuffix(strings.TrimPrefix(file, "catalog/entities/"), ".json")

		entity, err := f.entity(id)
		if err != nil {
			return "", err
		}

		_, inherited := entity.Data["ref"]
		inherited = inherited || len(collectRefs(entity.Data["identity"])) > 0

		if _, matched := identityScore(entity.Data, unknownFormFactor); matched || inherited {
			candidates = append(candidates, id)
		}
	}

	if len(candidates) == 0 {
		return "", ErrNoModelMatch
	}

	root, err := os.MkdirTemp("", "oddc-match-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(root)

	for _, candidate := range candidates {
		ids, err := f.closure(candidate)
		if err != nil {
			return "", err
		}

		if err := f.write(root, ids); err != nil {
			return "", err
		}
	}

	registry, err := LoadRegistry(root)
	if err != nil {
		return "", err
	}

	return registry.MatchModel(identity)
}

// answer writes model's reference closure, its evidence and the source
// revision to out, which must not exist yet. A failed fetch leaves no out.
func (f *fetcher) answer(model, out string) error {
	if _, err := os.Lstat(out); err == nil {
		return fmt.Errorf("%s already exists", out)
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}

	staging, err := os.MkdirTemp(filepath.Dir(out), ".oddc-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	ids, err := f.closure(model)
	if err != nil {
		return err
	}

	if err := f.write(staging, ids); err != nil {
		return err
	}

	evidence, err := f.source.List("evidence/" + model)
	if err != nil {
		return err
	}

	for _, file := range evidence {
		data, err := f.source.Read(file)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", file, err)
		}

		target := filepath.Join(staging, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}

	if err := os.WriteFile(
		filepath.Join(staging, "revision"),
		[]byte(f.source.Revision()+"\n"),
		0o644,
	); err != nil {
		return err
	}

	registry, err := LoadRegistry(staging)
	if err != nil {
		return err
	}
	if entity := registry.Entities[model]; entity.Kind != "DeviceModel" {
		return fmt.Errorf("%q is a %s, not a DeviceModel", model, entity.Kind)
	}

	if err := os.Chmod(staging, 0o755); err != nil {
		return err
	}

	return os.Rename(staging, out)
}
