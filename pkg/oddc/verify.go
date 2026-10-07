// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
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
