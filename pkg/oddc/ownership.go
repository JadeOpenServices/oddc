package oddc

import (
	"fmt"
	"strings"
)

func (r *Registry) traceOwnership(
	value any,
	prefix string,
	sourceID string,
	active map[string]bool,
	owners map[string]string,
) error {
	object, isObject := value.(map[string]any)
	if !isObject {
		if prefix != "" {
			owners[prefix] = sourceID
		}
		return nil
	}

	if ref, ok := object["ref"].(string); ok &&
		strings.TrimSpace(ref) != "" {
		if active[ref] {
			return fmt.Errorf(
				"entity reference cycle through %q",
				ref,
			)
		}

		target, exists := r.Entities[ref]
		if !exists {
			return fmt.Errorf(
				"%q references missing entity %q",
				sourceID,
				ref,
			)
		}

		active[ref] = true

		if err := r.traceOwnership(
			target.Data,
			prefix,
			ref,
			active,
			owners,
		); err != nil {
			return err
		}

		delete(active, ref)

		if prefix != "" {
			owners[prefix+".id"] = ref
			owners[prefix+".name"] = ref
			owners[prefix+".kind"] = ref
		}

		for key, child := range object {
			if key == "ref" {
				continue
			}

			childPath := key
			if prefix != "" {
				childPath = prefix + "." + key
			}

			clearOwners(owners, childPath)

			if err := r.traceOwnership(
				child,
				childPath,
				sourceID,
				active,
				owners,
			); err != nil {
				return err
			}
		}

		return nil
	}

	for key, child := range object {
		childPath := key
		if prefix != "" {
			childPath = prefix + "." + key
		}

		if err := r.traceOwnership(
			child,
			childPath,
			sourceID,
			active,
			owners,
		); err != nil {
			return err
		}
	}

	return nil
}

func clearOwners(
	owners map[string]string,
	path string,
) {
	prefix := path + "."

	for key := range owners {
		if key == path ||
			strings.HasPrefix(key, prefix) {
			delete(owners, key)
		}
	}
}
