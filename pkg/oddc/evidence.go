// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Evidence struct {
	Schema        string         `json:"$schema,omitempty"`
	SchemaVersion string         `json:"schemaVersion"`
	ID            string         `json:"id"`
	DeviceID      string         `json:"deviceId"`
	ObservedAt    string         `json:"observedAt"`
	Status        string         `json:"status"`
	Environment   map[string]any `json:"environment,omitempty"`
	Results       map[string]any `json:"results"`
	// Drivers are the kernel drivers bound to the model's components
	// that were present, by component ID.
	Drivers map[string][]string `json:"drivers,omitempty"`
	// Closure is the deployed model's closure the record was made on.
	Closure string `json:"closure,omitempty"`
	// InactiveQuirks are the model's kernel-ranged quirks the deployment
	// did not apply, as its inactive-quirks.json names them.
	InactiveQuirks []string `json:"inactiveQuirks,omitempty"`
}

// driverName is a kernel driver name such as "rtw89_8852be".
var driverName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// closureDigest is a Closure.
var closureDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ComponentDrivers names the drivers bound, as ReadDrivers lists them, to
// each component of a model that is present.
func (r *Registry) ComponentDrivers(model string, bound map[string]map[string][]string) (map[string][]string, error) {
	resolved, err := r.ResolveEntity(model)
	if err != nil {
		return nil, err
	}

	drivers := map[string][]string{}
	for _, component := range modelComponents(resolved.Resolved) {
		if names := bound[factsBus(component.Bus)][component.DeviceID]; len(names) > 0 {
			drivers[component.ID] = names
		}
	}

	return drivers, nil
}

func (r *Registry) validateEvidence() error {
	root := filepath.Join(r.Root, "evidence")

	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(
		root,
		func(
			path string,
			entry fs.DirEntry,
			walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() ||
				filepath.Ext(path) != ".json" {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			decoder := json.NewDecoder(file)
			decoder.UseNumber()
			decoder.DisallowUnknownFields()

			var evidence Evidence
			if err := decoder.Decode(&evidence); err != nil {
				return fmt.Errorf(
					"decode evidence %s: %w",
					path,
					err,
				)
			}

			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				if err == nil {
					return fmt.Errorf(
						"%s has trailing JSON",
						path,
					)
				}
				return err
			}

			if evidence.SchemaVersion != SchemaVersion {
				return fmt.Errorf(
					"%s schemaVersion=%q want=%q",
					path,
					evidence.SchemaVersion,
					SchemaVersion,
				)
			}

			if evidence.ID == "" ||
				evidence.DeviceID == "" ||
				evidence.ObservedAt == "" ||
				evidence.Status == "" ||
				evidence.Results == nil {
				return fmt.Errorf(
					"%s has incomplete evidence metadata",
					path,
				)
			}

			if problems := evidenceProblems(evidence); len(problems) > 0 {
				return fmt.Errorf(
					"%s may identify a machine or person: %s",
					path,
					strings.Join(problems, "; "),
				)
			}

			model, exists := r.Entities[evidence.DeviceID]
			if !exists {
				return fmt.Errorf(
					"%s references unknown model %q",
					path,
					evidence.DeviceID,
				)
			}

			// Evidence lives below its model: evidence/<deviceId>/.
			if filepath.Dir(path) != filepath.Join(root, filepath.FromSlash(evidence.DeviceID)) {
				return fmt.Errorf(
					"%s is about %q and belongs in %s",
					path,
					evidence.DeviceID,
					filepath.Join(root, filepath.FromSlash(evidence.DeviceID)),
				)
			}

			if model.Kind != "DeviceModel" {
				return fmt.Errorf(
					"%s references %q of kind %q, want DeviceModel",
					path,
					evidence.DeviceID,
					model.Kind,
				)
			}

			if evidence.Closure != "" && !closureDigest.MatchString(evidence.Closure) {
				return fmt.Errorf("%s: closure %q is not a sha256 digest", path, evidence.Closure)
			}

			resolved, err := r.ResolveEntity(evidence.DeviceID)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			ids := map[string]bool{}
			for _, component := range modelComponents(resolved.Resolved) {
				ids[component.ID] = true
			}
			for id, names := range evidence.Drivers {
				if !ids[id] {
					return fmt.Errorf("%s has drivers for %q, which is not a component of %s", path, id, evidence.DeviceID)
				}
				if len(names) == 0 {
					return fmt.Errorf("%s lists no driver for %q", path, id)
				}
				for _, name := range names {
					if !driverName.MatchString(name) {
						return fmt.Errorf("%s: %q is not a driver name", path, name)
					}
				}
			}

			// Passing evidence on the current closure must prove it; on an
			// older closure it proves nothing any more.
			closure, err := r.Closure(evidence.DeviceID)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if evidence.Closure == closure {
				unmet, err := r.Unmet(evidence.DeviceID, evidence)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				if len(unmet) > 0 {
					return fmt.Errorf("%s does not prove %s %s: %s", path, evidence.DeviceID, evidence.Status, strings.Join(unmet, "; "))
				}
			}

			return nil
		},
	)
}
