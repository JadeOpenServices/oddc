package oddc_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// nest turns a dotted path and value into a nested override object.
func nest(path string, value any) map[string]any {
	parts := strings.Split(path, ".")
	result := map[string]any{parts[len(parts)-1]: value}

	for i := len(parts) - 2; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}

	return result
}

func overlay(kind, modelID string, overrides map[string]any) Overlay {
	return Overlay{
		APIVersion:  EntityAPIVersion,
		ID:          kind + "/" + modelID,
		Kind:        kind,
		TargetModel: modelID,
		Overrides:   overrides,
	}
}

// Host overrides win over project overrides, which win over the catalog,
// and history keeps all three in order.
func TestCatalogOverridePrecedence(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		canonical, err := registry.ResolveModel(id, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		values := leavesOf(canonical.Resolved)
		paths := make([]string, 0, len(values))
		for path := range values {
			if !strings.HasPrefix(path, "model.") {
				paths = append(paths, path)
			}
		}
		sort.Strings(paths)

		// Each path takes, as overrides, the values of its neighbours, so
		// project and host differ from the catalog wherever the data does.
		for i, path := range paths {
			project := values[paths[(i+1)%len(paths)]]
			host := values[paths[(i+2)%len(paths)]]

			resolved, err := registry.ResolveModel(
				id,
				[]Overlay{overlay("project", id, nest(path, project))},
				[]Overlay{overlay("host", id, nest(path, host))},
			)
			if err != nil {
				t.Fatalf("%s %s: %v", id, path, err)
			}

			if got, _ := Lookup(resolved.Resolved, path); fmt.Sprint(got) != fmt.Sprint(host) {
				t.Errorf("%s: %s=%v want host %v", id, path, got, host)
			}

			var sources []string
			for _, record := range resolved.History[path] {
				sources = append(sources, record.Source)
			}

			want := []string{
				canonical.Provenance[path].Source,
				"project:project/" + id,
				"host:host/" + id,
			}
			if fmt.Sprint(sources) != fmt.Sprint(want) {
				t.Errorf("%s: %s history %v want %v", id, path, sources, want)
			}
		}
	}
}

// referencedNodes returns the resolved paths where a model mounts another
// entity.
func referencedNodes(resolved Resolved, modelID string) []string {
	var paths []string

	for path, record := range resolved.Provenance {
		if !strings.HasSuffix(path, ".id") || strings.HasPrefix(path, "model.") {
			continue
		}

		if record.Source != "catalog:"+modelID {
			paths = append(paths, strings.TrimSuffix(path, ".id"))
		}
	}

	sort.Strings(paths)
	return paths
}

// A host overlay can delete every node a model references, with a
// tombstone in history and the canonical history kept.
func TestCatalogHostDeletesEveryReferencedNode(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		canonical, err := registry.ResolveModel(id, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		for _, node := range referencedNodes(canonical, id) {
			host := overlay("host", id, nest(node, map[string]any{"$delete": true}))

			resolved, err := registry.ResolveModel(id, nil, []Overlay{host})
			if err != nil {
				t.Fatalf("%s %s: %v", id, node, err)
			}

			if _, exists := Lookup(resolved.Resolved, node); exists {
				t.Errorf("%s: %s survived deletion", id, node)
			}

			for path := range resolved.Provenance {
				if strings.HasPrefix(path, node+".") || path == node {
					t.Errorf("%s: %s kept provenance after deletion", id, path)
				}
			}

			history := resolved.History[node]
			if len(history) == 0 || history[len(history)-1].Source != "host:"+host.ID {
				t.Errorf("%s: %s has no deletion tombstone: %#v", id, node, history)
			}

			if len(resolved.History[node+".id"]) == 0 {
				t.Errorf("%s: %s lost its canonical history", id, node)
			}
		}
	}
}

func TestCatalogRejectsMalformedDeleteMarkers(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		canonical, err := registry.ResolveModel(id, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		for _, node := range referencedNodes(canonical, id) {
			kept, _ := Lookup(canonical.Resolved, node+".id")

			for _, marker := range []map[string]any{
				{"$delete": false},
				{"$delete": "true"},
				{"$delete": true, "id": kept},
			} {
				host := overlay("host", id, nest(node, marker))
				if _, err := registry.ResolveModel(id, nil, []Overlay{host}); err == nil {
					t.Errorf("%s: %s accepted delete marker %#v", id, node, marker)
				}
			}
		}
	}
}
