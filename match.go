package oddc

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
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

// ModelIdentity returns the machine identity a model declares: the first
// value of each DMI field and its class form factor. A machine reporting it
// matches the model.
func (r *Registry) ModelIdentity(id string) (MachineIdentity, error) {
	resolved, err := r.ResolveEntity(id)
	if err != nil {
		return MachineIdentity{}, err
	}

	first := func(path string) string {
		values := stringsAt(resolved.Resolved, path)
		if len(values) == 0 {
			return ""
		}

		return values[0]
	}

	formFactor, _ := stringAt(resolved.Resolved, "class.formFactor")

	return MachineIdentity{
		FormFactor:     formFactor,
		SysVendor:      first("identity.dmi.systemVendor"),
		ProductName:    first("identity.dmi.productName"),
		ProductVersion: first("identity.dmi.productVersion"),
		BoardVendor:    first("identity.dmi.boardVendor"),
		BoardName:      first("identity.dmi.boardName"),
		BoardVersion:   first("identity.dmi.boardVersion"),
	}, nil
}

// SysfsFiles returns the sysfs files, relative to the sysfs root, through
// which a machine reports this identity.
func (identity MachineIdentity) SysfsFiles() map[string]string {
	files := map[string]string{}

	for name, value := range map[string]string{
		"sys_vendor":      identity.SysVendor,
		"product_name":    identity.ProductName,
		"product_version": identity.ProductVersion,
		"board_vendor":    identity.BoardVendor,
		"board_name":      identity.BoardName,
		"board_version":   identity.BoardVersion,
	} {
		if value != "" {
			files[filepath.Join("class", "dmi", "id", name)] = value + "\n"
		}
	}

	switch identity.FormFactor {
	case "laptop":
		files[filepath.Join("class", "power_supply", "BAT0", "type")] = "Battery\n"
	case "desktop":
		files[filepath.Join("class", "dmi", "id", "chassis_type")] = "3\n"
	}

	return files
}
