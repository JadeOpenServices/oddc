// SPDX-License-Identifier: GPL-3.0-or-later

package oddc_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	. "github.com/JadeOpenServices/oddc/pkg/oddc"
)

// closureOf lists a model and every entity it references, transitively.
func closureOf(registry *Registry, id string) []string {
	seen := map[string]bool{}
	queue := []string{id}

	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if seen[next] {
			continue
		}
		seen[next] = true
		queue = append(queue, collectRefs(registry.Entities[next].Data)...)
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids
}

// filesBelow returns the slash-separated files below root, relative to it.
func filesBelow(t *testing.T, root string) []string {
	t.Helper()

	files, err := DirSource{Root: root}.List(".")
	if err != nil {
		t.Fatal(err)
	}
	for i, file := range files {
		files[i] = strings.TrimPrefix(file, "./")
	}
	sort.Strings(files)

	return files
}

// expectAnswer checks that out holds exactly model's closure and evidence,
// byte for byte as in the repository, and the revision.
func expectAnswer(t *testing.T, registry *Registry, model, out, revision string) {
	t.Helper()

	want := []string{"revision"}
	for _, id := range closureOf(registry, model) {
		want = append(want, "catalog/entities/"+id+".json")
	}
	evidence, err := DirSource{Root: "."}.List("evidence/" + model)
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, evidence...)
	sort.Strings(want)

	got := filesBelow(t, out)
	if !slices.Equal(got, want) {
		t.Fatalf("%s answer holds\n%v\nwant\n%v", model, got, want)
	}

	for _, file := range got {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}

		if file == "revision" {
			if string(data) != revision+"\n" {
				t.Errorf("%s revision %q, want %q", model, data, revision)
			}
			continue
		}

		source, err := os.ReadFile(filepath.FromSlash(file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, source) {
			t.Errorf("%s: %s differs from the repository", model, file)
		}
	}

	answer, err := LoadRegistry(out)
	if err != nil {
		t.Fatalf("%s answer invalid: %v", model, err)
	}
	if matched, err := answer.MatchModel(mustIdentity(t, registry, model)); err != nil || matched != model {
		t.Errorf("%s answer matches %q, %v", model, matched, err)
	}
}

func mustIdentity(t *testing.T, registry *Registry, model string) MachineIdentity {
	t.Helper()

	identity, err := registry.ModelIdentity(model)
	if err != nil {
		t.Fatal(err)
	}

	return identity
}

func TestFetchEveryModelFromCheckout(t *testing.T) {
	registry := catalog(t)

	for _, model := range models(t, registry) {
		out := filepath.Join(t.TempDir(), "oddc")

		matched, err := Fetch(DirSource{Root: "."}, mustIdentity(t, registry, model), out)
		if err != nil || matched != model {
			t.Fatalf("Fetch for %s = %q, %v", model, matched, err)
		}

		expectAnswer(t, registry, model, out, "local")
	}
}

func TestFetchModelByID(t *testing.T) {
	registry := catalog(t)

	for _, model := range models(t, registry) {
		out := filepath.Join(t.TempDir(), "oddc")

		if err := FetchModel(DirSource{Root: "."}, model, out); err != nil {
			t.Fatal(err)
		}

		expectAnswer(t, registry, model, out, "local")
	}
}

func TestFetchModelRefusesNonModels(t *testing.T) {
	registry := catalog(t)

	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			continue
		}

		out := filepath.Join(t.TempDir(), "oddc")
		if err := FetchModel(DirSource{Root: "."}, id, out); err == nil {
			t.Errorf("fetched %s, a %s, as a model", id, entity.Kind)
		}
		if _, err := os.Lstat(out); err == nil {
			t.Errorf("failed fetch of %s left %s", id, out)
		}
	}
}

func TestFetchUnknownMachineWritesNothing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "oddc")

	if _, err := Fetch(DirSource{Root: "."}, MachineIdentity{}, out); !errors.Is(err, ErrNoModelMatch) {
		t.Fatalf("empty identity: %v, want ErrNoModelMatch", err)
	}
	if _, err := os.Lstat(out); err == nil {
		t.Errorf("unmatched fetch left %s", out)
	}
}

func TestFetchKeepsExistingAnswer(t *testing.T) {
	registry := catalog(t)
	model := models(t, registry)[0]
	out := filepath.Join(t.TempDir(), "oddc")

	if err := FetchModel(DirSource{Root: "."}, model, out); err != nil {
		t.Fatal(err)
	}
	if err := FetchModel(DirSource{Root: "."}, model, out); err == nil {
		t.Error("fetch overwrote an existing answer")
	}
}
