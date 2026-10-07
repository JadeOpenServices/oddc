// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNoModelMatch        = errors.New("no ODDC model matched")
	ErrAmbiguousModelMatch = errors.New("ambiguous ODDC model match")
)

type MachineIdentity struct {
	FormFactor     string `json:"formFactor,omitempty"`
	SysVendor      string `json:"systemVendor,omitempty"`
	ProductName    string `json:"productName,omitempty"`
	ProductVersion string `json:"productVersion,omitempty"`
	BoardVendor    string `json:"boardVendor,omitempty"`
	BoardName      string `json:"boardName,omitempty"`
	BoardVersion   string `json:"boardVersion,omitempty"`
}

func (r *Registry) MatchModel(identity MachineIdentity) (string, error) {
	classification, err := r.Classify(Facts{Identity: identity})
	if err != nil {
		return "", err
	}

	switch classification.Result {
	case ResultMatched:
		return classification.Model, nil

	case ResultAmbiguous:
		return "", fmt.Errorf(
			"%w: %s",
			ErrAmbiguousModelMatch,
			strings.Join(classification.Ambiguous, ", "),
		)

	default:
		return "", ErrNoModelMatch
	}
}

// FieldMatch compares one identity field a model declares with what the
// machine reports.
type FieldMatch struct {
	Expected []string `json:"expected"`
	Actual   string   `json:"actual"`
	Matched  bool     `json:"matched"`
}

// identityFields compares every identity field a model's data declares,
// by its path in that data. The form factor is compared only when the
// machine reports one.
func identityFields(data map[string]any, identity MachineIdentity) map[string]FieldMatch {
	fields := map[string]FieldMatch{}

	if formFactor, ok := stringAt(
		data,
		"class.formFactor",
	); ok && strings.TrimSpace(identity.FormFactor) != "" {
		fields["class.formFactor"] = FieldMatch{
			Expected: []string{formFactor},
			Actual:   identity.FormFactor,
			Matched:  equalIdentity(formFactor, identity.FormFactor),
		}
	}

	checks := []struct {
		path   string
		actual string
	}{
		{"identity.dmi.systemVendor", identity.SysVendor},
		{"identity.dmi.productName", identity.ProductName},
		{"identity.dmi.productVersion", identity.ProductVersion},
		{"identity.dmi.boardVendor", identity.BoardVendor},
		{"identity.dmi.boardName", identity.BoardName},
		{"identity.dmi.boardVersion", identity.BoardVersion},
	}

	for _, check := range checks {
		expected := stringsAt(data, check.path)
		if len(expected) == 0 {
			continue
		}

		field := FieldMatch{Expected: expected, Actual: check.actual}
		for _, value := range expected {
			if equalIdentity(value, check.actual) {
				field.Matched = true
				break
			}
		}

		fields[check.path] = field
	}

	return fields
}

// identityScore counts the DMI fields a model's data declares and the
// machine reports. A model matches when it declares at least one field and
// none contradicts the machine.
func identityScore(data map[string]any, identity MachineIdentity) (int, bool) {
	score := 0

	for path, field := range identityFields(data, identity) {
		if !field.Matched {
			return 0, false
		}
		if path != "class.formFactor" {
			score++
		}
	}

	return score, score > 0
}

func (r *Registry) modelIDs() []string {
	result := make([]string, 0)

	for id, entity := range r.Entities {
		if entity.Kind == "DeviceModel" {
			result = append(result, id)
		}
	}

	return result
}
