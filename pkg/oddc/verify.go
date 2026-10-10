// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Verification states of a model.
const (
	// Verified: passing evidence was recorded on this closure.
	Verified = "verified"
	// Changed: passing evidence exists, but the closure changed since.
	Changed = "changed"
	// Unverified: no passing evidence names the closure it tested.
	Unverified = "unverified"
)

// passingStatuses are evidence statuses that prove a model on hardware.
var passingStatuses = []string{"runtime-verified", "hardware-validated"}

// Verification is how far a model's current closure is proven.
type Verification struct {
	Model   string `json:"model"`
	Status  string `json:"status"`
	Closure string `json:"closure"`
	// Evidence is the record the status rests on: the newest passing one
	// on this closure, else the newest passing one, else the newest one;
	// nil when the model has none.
	Evidence *Evidence `json:"evidence,omitempty"`
}

// Closure is a digest of a model's resolved catalog data: what a
// deployment of it is built from.
func (r *Registry) Closure(model string) (string, error) {
	resolved, err := r.ResolveEntity(model)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(resolved.Resolved)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ModelEvidence reads a model's evidence records, oldest first.
func (r *Registry) ModelEvidence(model string) ([]Evidence, error) {
	files, err := filepath.Glob(filepath.Join(r.Root, "evidence", filepath.FromSlash(model), "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	records := []Evidence{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		var record Evidence
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	// Several records of one day are named DATE, DATE-2, ...; the date
	// orders them first.
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ObservedAt < records[j].ObservedAt
	})

	return records, nil
}

// Verify tells whether a model's current closure is proven by evidence.
func (r *Registry) Verify(model string) (Verification, error) {
	closure, err := r.Closure(model)
	if err != nil {
		return Verification{}, err
	}

	records, err := r.ModelEvidence(model)
	if err != nil {
		return Verification{}, err
	}

	result := Verification{Model: model, Status: Unverified, Closure: closure}
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		if !slices.Contains(passingStatuses, record.Status) || record.Closure == "" {
			continue
		}

		if record.Closure == closure {
			unmet, err := r.Unmet(model, record)
			if err != nil {
				return Verification{}, err
			}
			if len(unmet) > 0 {
				continue
			}
			result.Status = Verified
			result.Evidence = &record
			break
		}
		if result.Evidence == nil {
			result.Status = Changed
			result.Evidence = &record
		}
	}

	if result.Evidence == nil && len(records) > 0 {
		result.Evidence = &records[len(records)-1]
	}

	return result, nil
}

// Requirements are the results hardware-validated evidence of a model
// holds, each with the value it must have: identity and every component,
// capability, quirk and policy section of the resolved model. A component
// is named by its path, as "hardware.network.wifi.primary"; the others as
// "capabilities.usb4". Each must be "pass", except a capability the model
// does not have, which must be "not-exposed", and a component its entity
// marks as unusable on Linux, which must be "unsupported": present, and
// not working.
func (r *Registry) Requirements(model string) (map[string]string, error) {
	resolved, err := r.ResolveEntity(model)
	if err != nil {
		return nil, err
	}

	required := map[string]string{"identity": "pass"}
	for _, component := range modelComponents(resolved.Resolved) {
		required[component.Path] = "pass"
	}
	for _, component := range UnsupportedComponents(resolved.Resolved) {
		required[component.Path] = "unsupported"
	}
	for _, section := range []string{"capabilities", "quirks", "policy"} {
		values, _ := resolved.Resolved[section].(map[string]any)
		for key, value := range values {
			required[section+"."+key] = "pass"
			if value == false {
				required[section+"."+key] = "not-exposed"
			}
		}
	}

	return required, nil
}

// Unmet lists what a record lacks to prove a model at its status, sorted.
// Runtime-verified evidence was made on a deployment of the model, has
// identity "pass" and every driver the catalog names for a component
// bound to it. Hardware-validated evidence also has every requirement.
// A kernel-ranged quirk the deployment did not apply, as the record's
// InactiveQuirks names it, must be "not-affected" instead of "pass".
// Other statuses prove nothing and need nothing.
func (r *Registry) Unmet(model string, record Evidence) ([]string, error) {
	if !slices.Contains(passingStatuses, record.Status) {
		return nil, nil
	}

	resolved, err := r.ResolveEntity(model)
	if err != nil {
		return nil, err
	}

	var unmet []string
	if record.Closure == "" {
		unmet = append(unmet, "closure: not recorded on a deployment of the model")
	}

	required := map[string]string{"identity": "pass"}
	if record.Status == "hardware-validated" {
		if required, err = r.Requirements(model); err != nil {
			return nil, err
		}
	}
	for _, key := range record.InactiveQuirks {
		if componentAt(resolved.Resolved, "quirks."+key+".affected.kernel") == nil {
			unmet = append(unmet, fmt.Sprintf("quirk %s: not kernel-ranged, so always applied", key))
			continue
		}
		if _, ok := required["quirks."+key]; ok {
			required["quirks."+key] = "not-affected"
		}
	}
	for name, want := range required {
		got, recorded := record.Results[name]
		switch {
		case !recorded:
			unmet = append(unmet, fmt.Sprintf("result %s: missing, want %s", name, want))
		case got != want:
			unmet = append(unmet, fmt.Sprintf("result %s: %v, want %s", name, got, want))
		}
	}

	for _, component := range modelComponents(resolved.Resolved) {
		driver, _ := componentAt(resolved.Resolved, component.Path)["driver"].(string)
		if driver != "" && !slices.Contains(record.Drivers[component.ID], driver) {
			unmet = append(unmet, fmt.Sprintf("driver %s: not bound to %s", driver, component.Path))
		}
	}

	sort.Strings(unmet)
	return unmet, nil
}

// componentAt is the object at a dotted path of the resolved model, or nil.
func componentAt(data map[string]any, path string) map[string]any {
	for _, key := range strings.Split(path, ".") {
		data, _ = data[key].(map[string]any)
	}
	return data
}
