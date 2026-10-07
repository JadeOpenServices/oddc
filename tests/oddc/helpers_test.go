package oddc_test

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// The library keeps these to itself; the tests see the catalog through
// its files and its exported API.

// collectRefs lists every reference below value, sorted.
func collectRefs(value any) []string {
	set := map[string]bool{}

	var walk func(any)
	walk = func(current any) {
		object, ok := current.(map[string]any)
		if !ok {
			return
		}
		if ref, ok := object["ref"].(string); ok && ref != "" {
			set[ref] = true
		}
		for _, child := range object {
			walk(child)
		}
	}
	walk(value)

	refs := make([]string, 0, len(set))
	for ref := range set {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	return refs
}

// modelIDs lists the registry's models.
func modelIDs(registry *Registry) []string {
	var ids []string
	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			ids = append(ids, id)
		}
	}

	return ids
}

func decodeEntity(path string) (Entity, error) {
	var entity Entity

	data, err := os.ReadFile(path)
	if err != nil {
		return entity, err
	}

	return entity, json.Unmarshal(data, &entity)
}

// plausible tells whether a model's own data could be the machine: it
// declares at least one DMI field and the machine reports each as declared.
func plausible(data map[string]any, identity MachineIdentity) bool {
	declared := 0

	for path, actual := range map[string]string{
		"systemVendor":   identity.SysVendor,
		"productName":    identity.ProductName,
		"productVersion": identity.ProductVersion,
		"boardVendor":    identity.BoardVendor,
		"boardName":      identity.BoardName,
		"boardVersion":   identity.BoardVersion,
	} {
		value, ok := Lookup(data, "identity.dmi."+path)
		if !ok {
			continue
		}

		var expected []string
		switch typed := value.(type) {
		case string:
			expected = []string{typed}
		case map[string]any:
			for _, item := range typed {
				if text, ok := item.(string); ok {
					expected = append(expected, text)
				}
			}
		}
		if len(expected) == 0 {
			continue
		}

		declared++
		matched := false
		for _, want := range expected {
			matched = matched || strings.EqualFold(strings.TrimSpace(want), strings.TrimSpace(actual))
		}
		if !matched {
			return false
		}
	}

	return declared > 0
}
