// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import "fmt"

func (r *Registry) ResolveEntity(id string) (ResolvedEntity, error) {
	entity, exists := r.Entities[id]
	if !exists {
		return ResolvedEntity{}, fmt.Errorf("unknown entity %q", id)
	}

	active := map[string]bool{}

	resolved, err := r.expandObject(entity.Data, active, id)
	if err != nil {
		return ResolvedEntity{}, err
	}

	return ResolvedEntity{
		ID:         entity.Metadata.ID,
		Name:       entity.Metadata.Name,
		Kind:       entity.Kind,
		Resolved:   resolved,
		References: collectRefs(entity.Data),
	}, nil
}

func (r *Registry) expandObject(
	object map[string]any,
	active map[string]bool,
	context string,
) (map[string]any, error) {
	if ref, ok := object["ref"].(string); ok && ref != "" {
		if active[ref] {
			return nil, fmt.Errorf(
				"entity reference cycle while resolving %q through %q",
				context,
				ref,
			)
		}

		target, exists := r.Entities[ref]
		if !exists {
			return nil, fmt.Errorf(
				"%q references missing entity %q",
				context,
				ref,
			)
		}

		active[ref] = true

		base, err := r.expandObject(target.Data, active, ref)
		if err != nil {
			return nil, err
		}

		delete(active, ref)

		result := cloneObject(base)
		result["id"] = target.Metadata.ID
		result["name"] = target.Metadata.Name

		for key, value := range object {
			if key == "ref" {
				continue
			}

			expanded, err := r.expandValue(value, active, context)
			if err != nil {
				return nil, err
			}

			result[key] = expanded
		}

		return result, nil
	}

	result := make(map[string]any, len(object))

	for key, value := range object {
		expanded, err := r.expandValue(value, active, context)
		if err != nil {
			return nil, err
		}

		result[key] = expanded
	}

	return result, nil
}

func (r *Registry) expandValue(
	value any,
	active map[string]bool,
	context string,
) (any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	return r.expandObject(object, active, context)
}

func cloneObject(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))

	for key, value := range source {
		object, ok := value.(map[string]any)
		if ok {
			result[key] = cloneObject(object)
			continue
		}

		result[key] = value
	}

	return result
}
