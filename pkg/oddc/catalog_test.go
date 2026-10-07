package oddc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every test runs over the real catalog and takes its expectations from
// the catalog and evidence. Device facts are never repeated here.

func catalog(t *testing.T) *Registry {
	t.Helper()

	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}

	return registry
}

func models(t *testing.T, registry *Registry) []string {
	t.Helper()

	ids := registry.modelIDs()
	sort.Strings(ids)

	if len(ids) == 0 {
		t.Fatal("catalog has no models")
	}

	return ids
}

// leaves returns every scalar value below root by path.
func leaves(root map[string]any, prefix string, result map[string]any) {
	for key, value := range root {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if object, ok := value.(map[string]any); ok {
			leaves(object, path, result)
			continue
		}

		result[path] = value
	}
}

func leavesOf(root map[string]any) map[string]any {
	result := map[string]any{}
	leaves(root, "", result)
	return result
}

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

func TestCatalogEveryModelResolves(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		if _, err := registry.ResolveModel(id, nil, nil); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
}

// Every resolved value names its owner, and the owner's own file holds it.
func TestCatalogEveryValueHasItsOwner(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		resolved, err := registry.ResolveModel(id, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		for path, value := range leavesOf(resolved.Resolved) {
			record, exists := resolved.Provenance[path]
			if !exists {
				t.Errorf("%s: %s has no owner", id, path)
				continue
			}

			owner, exists := registry.Entities[strings.TrimPrefix(record.Source, "catalog:")]
			if !exists {
				t.Errorf("%s: %s owned by unknown %q", id, path, record.Source)
				continue
			}

			if !ownerHolds(owner, path, value) {
				t.Errorf("%s: %s=%v not in owner %s", id, path, value, owner.Metadata.ID)
			}
		}
	}
}

// ownerHolds reports whether the owner's data holds value at a suffix of
// path, or path ends in the identity the resolver adds for a reference.
func ownerHolds(owner Entity, path string, value any) bool {
	parts := strings.Split(path, ".")

	switch parts[len(parts)-1] {
	case "id":
		if value == owner.Metadata.ID {
			return true
		}
	case "name":
		if value == owner.Metadata.Name {
			return true
		}
	case "kind":
		if value == owner.Kind {
			return true
		}
	}

	for i := range parts {
		if held, ok := Lookup(owner.Data, strings.Join(parts[i:], ".")); ok &&
			fmt.Sprint(held) == fmt.Sprint(value) {
			return true
		}
	}

	return false
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

// Every model declares an identity, and exactly that model matches it.
func TestCatalogEveryModelMatchesOnlyItself(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		identity, err := registry.ModelIdentity(id)
		if err != nil {
			t.Fatal(err)
		}

		if identity.SysVendor == "" && identity.ProductName == "" && identity.BoardName == "" {
			t.Errorf("%s declares no DMI identity", id)
			continue
		}

		matched, err := registry.MatchModel(identity)
		if err != nil || matched != id {
			t.Errorf("%s: own identity matched %q, err %v", id, matched, err)
		}
	}
}

func TestCatalogEmptyIdentityMatchesNothing(t *testing.T) {
	_, err := catalog(t).MatchModel(MachineIdentity{})
	if !errors.Is(err, ErrNoModelMatch) {
		t.Fatalf("err = %v", err)
	}
}

// A machine reporting a model's identity through sysfs is read back as
// that model.
func TestCatalogReadIdentityFromSysfs(t *testing.T) {
	registry := catalog(t)

	for _, id := range models(t, registry) {
		identity, err := registry.ModelIdentity(id)
		if err != nil {
			t.Fatal(err)
		}

		sys := t.TempDir()
		for name, content := range identity.SysfsFiles() {
			path := filepath.Join(sys, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		if got := ReadIdentity(sys); got != identity {
			t.Errorf("%s: read %+v want %+v", id, got, identity)
		}
	}
}

// Evidence that passed identity belongs to a model that declares one.
func TestCatalogEvidenceIdentityPass(t *testing.T) {
	registry := catalog(t)

	err := filepath.WalkDir("evidence", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var evidence Evidence
		if err := json.Unmarshal(data, &evidence); err != nil {
			return err
		}

		if evidence.Results["identity"] != "pass" {
			return nil
		}

		identity, err := registry.ModelIdentity(evidence.DeviceID)
		if err != nil {
			return err
		}

		if identity.SysVendor == "" && identity.ProductName == "" {
			t.Errorf("%s passed identity, but %s declares none", path, evidence.DeviceID)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCatalogValidates(t *testing.T) {
	registry := catalog(t)

	result := Validate(".")
	if !result.Valid || len(result.Errors) != 0 {
		t.Fatalf("catalog invalid: %+v", result)
	}
	if result.Entities != len(registry.Entities) {
		t.Errorf("entities = %d, want %d", result.Entities, len(registry.Entities))
	}
	if result.Evidence != len(evidenceFiles(t)) {
		t.Errorf("evidence = %d, want %d", result.Evidence, len(evidenceFiles(t)))
	}
}

// The index names every entity at the file that holds it, and every
// evidence record under its model.
func TestCatalogIndex(t *testing.T) {
	registry := catalog(t)

	index, err := registry.Index("test")
	if err != nil {
		t.Fatal(err)
	}

	if len(index.Entities) != len(registry.Entities) {
		t.Fatalf("index has %d entities, want %d", len(index.Entities), len(registry.Entities))
	}

	var evidence []string
	for id, entry := range index.Entities {
		entity, err := decodeEntity(filepath.FromSlash(entry.Path))
		if err != nil {
			t.Fatal(err)
		}
		if entity.Metadata.ID != id || entity.Kind != entry.Kind {
			t.Errorf("%s: index points at %s holding %s", id, entry.Path, entity.Metadata.ID)
		}
		for _, file := range entry.Evidence {
			evidence = append(evidence, filepath.FromSlash(file))
		}
	}

	sort.Strings(evidence)
	want := evidenceFiles(t)
	sort.Strings(want)
	if strings.Join(evidence, ",") != strings.Join(want, ",") {
		t.Errorf("index evidence %v, want %v", evidence, want)
	}
}
