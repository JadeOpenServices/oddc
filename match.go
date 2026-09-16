package oddc

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrNoModelMatch        = errors.New("no ODDC model matched")
	ErrAmbiguousModelMatch = errors.New("ambiguous ODDC model match")
)

type MachineIdentity struct {
	FormFactor     string
	SysVendor      string
	ProductName    string
	ProductVersion string
	BoardVendor    string
	BoardName      string
	BoardVersion   string
}

func (r *Registry) MatchModel(identity MachineIdentity) (string, error) {
	ids := append([]string(nil), r.modelIDs()...)
	sort.Strings(ids)

	bestScore := -1
	var best []string

	for _, id := range ids {
		resolved, err := r.ResolveEntity(id)
		if err != nil {
			return "", err
		}

		if formFactor, ok := stringAt(
			resolved.Resolved,
			"class.formFactor",
		); ok && strings.TrimSpace(identity.FormFactor) != "" &&
			!equalIdentity(formFactor, identity.FormFactor) {
			continue
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

		score := 0
		matched := true

		for _, check := range checks {
			expected := stringsAt(
				resolved.Resolved,
				check.path,
			)

			if len(expected) == 0 {
				continue
			}

			found := false
			for _, value := range expected {
				if equalIdentity(value, check.actual) {
					found = true
					break
				}
			}

			if !found {
				matched = false
				break
			}

			score++
		}

		if !matched || score == 0 {
			continue
		}

		switch {
		case score > bestScore:
			bestScore = score
			best = []string{id}

		case score == bestScore:
			best = append(best, id)
		}
	}

	switch len(best) {
	case 0:
		return "", ErrNoModelMatch

	case 1:
		return best[0], nil

	default:
		return "", fmt.Errorf(
			"%w: %s",
			ErrAmbiguousModelMatch,
			strings.Join(best, ", "),
		)
	}
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

func stringsAt(root map[string]any, path string) []string {
	value, exists := Lookup(root, path)
	if !exists {
		return nil
	}

	switch typed := value.(type) {
	case string:
		return []string{typed}

	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		result := make([]string, 0, len(keys))

		for _, key := range keys {
			if item, ok := typed[key].(string); ok {
				result = append(result, item)
			}
		}

		return result

	default:
		return nil
	}
}

func stringAt(
	root map[string]any,
	path string,
) (string, bool) {
	value, exists := Lookup(root, path)
	if !exists {
		return "", false
	}

	result, ok := value.(string)
	return result, ok
}

func equalIdentity(a, b string) bool {
	return strings.EqualFold(
		strings.TrimSpace(a),
		strings.TrimSpace(b),
	)
}
