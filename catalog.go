package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = "2.0.0"

type Source struct {
	Type       string `json:"type"`
	Reference  string `json:"reference"`
	ObservedAt string `json:"observedAt,omitempty"`
}

type Document struct {
	Schema        string            `json:"$schema,omitempty"`
	SchemaVersion string            `json:"schemaVersion"`
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	Extends       []string          `json:"extends,omitempty"`
	Facts         map[string]any    `json:"facts,omitempty"`
	Blocks        map[string]any    `json:"blocks,omitempty"`
	Sources       map[string]Source `json:"sources,omitempty"`
}

type Overlay struct {
	Schema        string         `json:"$schema,omitempty"`
	SchemaVersion string         `json:"schemaVersion"`
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	TargetDevice  string         `json:"targetDevice"`
	Blocks        map[string]any `json:"blocks"`
}

type Provenance struct {
	Source string `json:"source"`
	Value  any    `json:"value"`
}

type Resolved struct {
	DeviceID   string                  `json:"deviceId"`
	Layers     []string                `json:"layers"`
	Blocks     map[string]any          `json:"blocks"`
	Provenance map[string]Provenance   `json:"provenance"`
	History    map[string][]Provenance `json:"history"`
}

type Catalog struct {
	Root      string
	Documents map[string]Document
	paths     map[string]string
}

func Load(root string) (*Catalog, error) {
	c := &Catalog{
		Root:      root,
		Documents: map[string]Document{},
		paths:     map[string]string{},
	}

	catalogRoot := filepath.Join(root, "catalog")

	err := filepath.WalkDir(
		catalogRoot,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}

			doc, err := decodeDocument(path)
			if err != nil {
				return err
			}
			if err := validateDocument(doc); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}

			if previous, exists := c.paths[doc.ID]; exists {
				return fmt.Errorf(
					"duplicate catalog id %q in %s and %s",
					doc.ID,
					previous,
					path,
				)
			}

			c.Documents[doc.ID] = doc
			c.paths[doc.ID] = path
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}

	for id, doc := range c.Documents {
		for _, parent := range doc.Extends {
			if _, exists := c.Documents[parent]; !exists {
				return nil, fmt.Errorf(
					"%q extends missing layer %q",
					id,
					parent,
				)
			}
		}
	}

	if err := c.validateCycles(); err != nil {
		return nil, err
	}

	if err := c.validateEvidence(); err != nil {
		return nil, err
	}

	return c, nil
}

func decodeDocument(path string) (Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var doc Document
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode %s: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Document{}, fmt.Errorf("%s has trailing JSON", path)
		}
		return Document{}, err
	}

	return doc, nil
}

func validateDocument(doc Document) error {
	if doc.SchemaVersion != SchemaVersion {
		return fmt.Errorf(
			"schemaVersion=%q want=%q",
			doc.SchemaVersion,
			SchemaVersion,
		)
	}

	if strings.TrimSpace(doc.ID) == "" {
		return fmt.Errorf("id is required")
	}

	switch doc.Kind {
	case "class", "vendor", "family", "device":
	default:
		return fmt.Errorf("invalid kind %q", doc.Kind)
	}

	if doc.Blocks == nil {
		doc.Blocks = map[string]any{}
	}

	return rejectArrays(doc.Blocks, "blocks")
}

func rejectArrays(value any, path string) error {
	switch typed := value.(type) {
	case []any:
		return fmt.Errorf(
			"%s is positional; overrideable blocks must use keyed objects",
			path,
		)

	case map[string]any:
		for key, child := range typed {
			next := key
			if path != "" {
				next = path + "." + key
			}
			if err := rejectArrays(child, next); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Catalog) validateCycles() error {
	done := map[string]bool{}
	active := map[string]bool{}

	var visit func(string) error
	visit = func(id string) error {
		if active[id] {
			return fmt.Errorf("catalog inheritance cycle at %q", id)
		}
		if done[id] {
			return nil
		}

		active[id] = true

		for _, parent := range c.Documents[id].Extends {
			if err := visit(parent); err != nil {
				return err
			}
		}

		delete(active, id)
		done[id] = true
		return nil
	}

	for id := range c.Documents {
		if err := visit(id); err != nil {
			return err
		}
	}

	return nil
}

func (c *Catalog) layers(deviceID string) ([]Document, error) {
	device, exists := c.Documents[deviceID]
	if !exists {
		return nil, fmt.Errorf("unknown device %q", deviceID)
	}
	if device.Kind != "device" {
		return nil, fmt.Errorf(
			"%q is kind %q, not device",
			deviceID,
			device.Kind,
		)
	}

	seen := map[string]bool{}
	var ordered []Document

	var appendLayer func(string) error
	appendLayer = func(id string) error {
		if seen[id] {
			return nil
		}

		doc := c.Documents[id]

		for _, parent := range doc.Extends {
			if err := appendLayer(parent); err != nil {
				return err
			}
		}

		seen[id] = true
		ordered = append(ordered, doc)
		return nil
	}

	if err := appendLayer(deviceID); err != nil {
		return nil, err
	}

	return ordered, nil
}

func (c *Catalog) Resolve(
	deviceID string,
	project []Overlay,
	host []Overlay,
) (Resolved, error) {
	layers, err := c.layers(deviceID)
	if err != nil {
		return Resolved{}, err
	}

	result := Resolved{
		DeviceID:   deviceID,
		Blocks:     map[string]any{},
		Provenance: map[string]Provenance{},
		History:    map[string][]Provenance{},
	}

	for _, layer := range layers {
		result.Layers = append(result.Layers, layer.ID)

		merge(
			result.Blocks,
			layer.Blocks,
			"",
			"catalog:"+layer.ID,
			&result,
		)
	}

	for _, overlay := range project {
		if err := validateOverlay(overlay, deviceID, "project"); err != nil {
			return Resolved{}, err
		}

		result.Layers = append(
			result.Layers,
			"project:"+overlay.ID,
		)

		merge(
			result.Blocks,
			overlay.Blocks,
			"",
			"project:"+overlay.ID,
			&result,
		)
	}

	for _, overlay := range host {
		if err := validateOverlay(overlay, deviceID, "host"); err != nil {
			return Resolved{}, err
		}

		result.Layers = append(
			result.Layers,
			"host:"+overlay.ID,
		)

		merge(
			result.Blocks,
			overlay.Blocks,
			"",
			"host:"+overlay.ID,
			&result,
		)
	}

	return result, nil
}

func ReadOverlay(path string) (Overlay, error) {
	file, err := os.Open(path)
	if err != nil {
		return Overlay{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var overlay Overlay
	if err := decoder.Decode(&overlay); err != nil {
		return Overlay{}, fmt.Errorf("decode overlay %s: %w", path, err)
	}

	if err := rejectArrays(overlay.Blocks, "blocks"); err != nil {
		return Overlay{}, fmt.Errorf("%s: %w", path, err)
	}

	return overlay, nil
}

func validateOverlay(
	overlay Overlay,
	deviceID string,
	wantKind string,
) error {
	if overlay.SchemaVersion != SchemaVersion {
		return fmt.Errorf(
			"overlay %q schemaVersion=%q want=%q",
			overlay.ID,
			overlay.SchemaVersion,
			SchemaVersion,
		)
	}

	if overlay.Kind != wantKind {
		return fmt.Errorf(
			"overlay %q kind=%q want=%q",
			overlay.ID,
			overlay.Kind,
			wantKind,
		)
	}

	if overlay.TargetDevice != deviceID {
		return fmt.Errorf(
			"overlay %q targets %q, resolving %q",
			overlay.ID,
			overlay.TargetDevice,
			deviceID,
		)
	}

	return rejectArrays(overlay.Blocks, "blocks")
}

func merge(
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

		incoming, isObject := value.(map[string]any)
		if isObject {
			current, ok := dst[key].(map[string]any)
			if !ok {
				current = map[string]any{}
				dst[key] = current
				clearProvenance(result, path)
			}

			merge(
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

func clearProvenance(result *Resolved, path string) {
	prefix := path + "."

	for key := range result.Provenance {
		if key == path || strings.HasPrefix(key, prefix) {
			delete(result.Provenance, key)
		}
	}
}

func Lookup(root map[string]any, path string) (any, bool) {
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

func (c *Catalog) validateEvidence() error {
	root := filepath.Join(c.Root, "evidence")

	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(
		root,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var evidence struct {
				SchemaVersion string         `json:"schemaVersion"`
				ID            string         `json:"id"`
				DeviceID      string         `json:"deviceId"`
				ObservedAt    string         `json:"observedAt"`
				Status        string         `json:"status"`
				Results       map[string]any `json:"results"`
			}

			if err := json.Unmarshal(data, &evidence); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}

			if evidence.SchemaVersion != SchemaVersion {
				return fmt.Errorf(
					"%s has unsupported evidence schema",
					path,
				)
			}

			if evidence.ID == "" ||
				evidence.DeviceID == "" ||
				evidence.ObservedAt == "" ||
				evidence.Status == "" ||
				evidence.Results == nil {
				return fmt.Errorf(
					"%s has incomplete evidence metadata",
					path,
				)
			}

			if _, exists := c.Documents[evidence.DeviceID]; !exists {
				return fmt.Errorf(
					"%s references unknown device %q",
					path,
					evidence.DeviceID,
				)
			}

			return nil
		},
	)
}
