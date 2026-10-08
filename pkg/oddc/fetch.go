// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// fetcher reads entities from a source at most once.
type fetcher struct {
	source   Source
	entities map[string][]byte
}

func newFetcher(source Source) *fetcher {
	return &fetcher{source: source, entities: map[string][]byte{}}
}

// entity reads an entity by ID from its address.
func (f *fetcher) entity(id string) (Entity, error) {
	file := "catalog/entities/" + id + ".json"

	data, cached := f.entities[id]
	if !cached {
		var err error
		if data, err = f.source.Read(file); err != nil {
			return Entity{}, fmt.Errorf("fetch %s: %w", id, err)
		}
	}

	entity, err := decodeEntityData(file, data)
	if err != nil {
		return Entity{}, err
	}

	f.entities[id] = data
	return entity, nil
}

// closure returns id and every entity it references, transitively.
func (f *fetcher) closure(id string) ([]string, error) {
	seen := map[string]bool{id: true}
	queue := []string{id}

	for len(queue) > 0 {
		entity, err := f.entity(queue[0])
		if err != nil {
			return nil, err
		}
		queue = queue[1:]

		for _, ref := range collectRefs(entity.Data) {
			if !seen[ref] {
				seen[ref] = true
				queue = append(queue, ref)
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids, nil
}

// write puts the entities, in canonical layout, below root.
func (f *fetcher) write(root string, ids []string) error {
	for _, id := range ids {
		file := EntityPath(filepath.Join(root, "catalog", "entities"), id)

		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, f.entities[id], 0o644); err != nil {
			return err
		}
	}

	return nil
}

// Match returns the model a machine with this identity is, reading only
// the source's model files and the references of plausible models.
func Match(source Source, identity MachineIdentity) (string, error) {
	return newFetcher(source).match(identity)
}

// Fetch matches the machine and writes only its model to out: the model's
// reference closure in canonical layout, its evidence and the revision.
func Fetch(source Source, identity MachineIdentity, out string) (string, error) {
	f := newFetcher(source)

	model, err := f.match(identity)
	if err != nil {
		return "", err
	}

	return model, f.answer(model, out)
}

// FetchModel writes one model, chosen by ID, to out as Fetch does.
func FetchModel(source Source, model, out string) error {
	return newFetcher(source).answer(model, out)
}
