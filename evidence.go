package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Evidence struct {
	Schema        string         `json:"$schema,omitempty"`
	SchemaVersion string         `json:"schemaVersion"`
	ID            string         `json:"id"`
	DeviceID      string         `json:"deviceId"`
	ObservedAt    string         `json:"observedAt"`
	Status        string         `json:"status"`
	Environment   map[string]any `json:"environment,omitempty"`
	Results       map[string]any `json:"results"`
}

func (r *Registry) validateEvidence() error {
	root := filepath.Join(r.Root, "evidence")

	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(
		root,
		func(
			path string,
			entry fs.DirEntry,
			walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() ||
				filepath.Ext(path) != ".json" {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			decoder := json.NewDecoder(file)
			decoder.UseNumber()
			decoder.DisallowUnknownFields()

			var evidence Evidence
			if err := decoder.Decode(&evidence); err != nil {
				return fmt.Errorf(
					"decode evidence %s: %w",
					path,
					err,
				)
			}

			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				if err == nil {
					return fmt.Errorf(
						"%s has trailing JSON",
						path,
					)
				}
				return err
			}

			if evidence.SchemaVersion != SchemaVersion {
				return fmt.Errorf(
					"%s schemaVersion=%q want=%q",
					path,
					evidence.SchemaVersion,
					SchemaVersion,
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

			model, exists := r.Entities[evidence.DeviceID]
			if !exists {
				return fmt.Errorf(
					"%s references unknown model %q",
					path,
					evidence.DeviceID,
				)
			}

			if model.Kind != "DeviceModel" {
				return fmt.Errorf(
					"%s references %q of kind %q, want DeviceModel",
					path,
					evidence.DeviceID,
					model.Kind,
				)
			}

			return nil
		},
	)
}
