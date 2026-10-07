// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"sort"
	"strings"
)

func mergeResolved(
	dst map[string]any,
	src map[string]any,
	prefix string,
	source string,
	result *Resolved,
) {
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := src[key]

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if deleteMarker(value) {
			clearProvenance(result, path)
			delete(dst, key)

			record := Provenance{
				Source: source,
				Value: map[string]any{
					"$delete": true,
				},
			}

			result.History[path] = append(
				result.History[path],
				record,
			)

			continue
		}

		incoming, isObject := value.(map[string]any)
		if isObject {
			current, ok := dst[key].(map[string]any)
			if !ok {
				current = map[string]any{}
				dst[key] = current
				clearProvenance(result, path)
			}

			mergeResolved(
				current,
				incoming,
				path,
				source,
				result,
			)
			continue
		}

		clearProvenance(result, path)

		dst[key] = value

		record := Provenance{
			Source: source,
			Value:  value,
		}

		result.Provenance[path] = record
		result.History[path] = append(
			result.History[path],
			record,
		)
	}
}

func clearProvenance(
	result *Resolved,
	path string,
) {
	prefix := path + "."

	for key := range result.Provenance {
		if key == path ||
			strings.HasPrefix(key, prefix) {
			delete(result.Provenance, key)
		}
	}
}

func Lookup(
	root map[string]any,
	path string,
) (any, bool) {
	if path == "" {
		return root, true
	}

	var current any = root

	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}
