// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Duplication finds what CONTRIBUTING.md section 18 forbids: facts or
// behaviour held in more than one place. Each problem names the entities
// and says where the value belongs instead.
//
//   - A model's quirk is a reference to a Quirk entity, plus "enabled";
//     the quirk's data lives only in that entity.
//   - No two Quirk entities hold the same data. A behaviour several
//     models need is one quirk they all reference, each model proving it
//     with its own evidence.
//   - A component inside a model (an object with a bus and a deviceId) is
//     a reference to a component entity, never copied in.
//   - No two component entities claim the same bus and deviceId.
func (r *Registry) Duplication() []string {
	var problems []string

	ids := make([]string, 0, len(r.Entities))
	for id := range r.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	quirkData := map[string]string{}
	deviceOwner := map[string]string{}

	for _, id := range ids {
		entity := r.Entities[id]
		switch {
		case entity.Kind == "Quirk":
			key := canonical(entity.Data)
			if other, seen := quirkData[key]; seen {
				problems = append(problems, fmt.Sprintf(
					"%s and %s hold the same quirk data; keep one and reference it from every model that needs it",
					other, id))
			} else {
				quirkData[key] = id
			}

		case entity.Kind == "DeviceModel":
			problems = append(problems, modelQuirkProblems(id, entity.Data)...)
			if hardware, ok := entity.Data["hardware"].(map[string]any); ok {
				problems = append(problems, inlineComponents(id, "hardware", hardware)...)
			}

		case strings.HasSuffix(entity.Kind, "Component"):
			bus, _ := entity.Data["bus"].(string)
			device, _ := entity.Data["deviceId"].(string)
			if bus == "" || device == "" {
				continue
			}
			key := bus + " " + strings.ToLower(device)
			if other, seen := deviceOwner[key]; seen {
				problems = append(problems, fmt.Sprintf(
					"%s and %s both describe %s device %s; one component entity owns a device",
					other, id, bus, device))
			} else {
				deviceOwner[key] = id
			}
		}
	}

	return problems
}

// modelQuirkProblems reports quirks a model holds itself instead of
// referencing.
func modelQuirkProblems(model string, data map[string]any) []string {
	quirks, _ := data["quirks"].(map[string]any)

	keys := make([]string, 0, len(quirks))
	for key := range quirks {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var problems []string
	for _, key := range keys {
		quirk, _ := quirks[key].(map[string]any)
		if ref, _ := quirk["ref"].(string); ref == "" {
			problems = append(problems, fmt.Sprintf(
				"%s quirks.%s has no ref; put the quirk in a quirk entity and reference it", model, key))
			continue
		}
		for field := range quirk {
			if field != "ref" && field != "enabled" {
				problems = append(problems, fmt.Sprintf(
					"%s quirks.%s sets %q; a model only references a quirk and enables it, the quirk entity holds its data",
					model, key, field))
			}
		}
	}
	return problems
}

// inlineComponents reports objects below a model's hardware section that
// carry a device identity without referencing a component entity.
func inlineComponents(model, path string, value map[string]any) []string {
	if _, isRef := value["ref"]; isRef {
		return nil
	}
	if _, bus := value["bus"].(string); bus {
		if _, device := value["deviceId"].(string); device {
			return []string{fmt.Sprintf(
				"%s %s describes a %v device itself; make it a component entity and reference it",
				model, path, value["bus"])}
		}
	}

	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var problems []string
	for _, key := range keys {
		if child, ok := value[key].(map[string]any); ok {
			problems = append(problems, inlineComponents(model, path+"."+key, child)...)
		}
	}
	return problems
}

// canonical renders a value with sorted keys, so equal data compares
// equal however it was written.
func canonical(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
