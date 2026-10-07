package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Overlay struct {
	Schema      string         `json:"$schema,omitempty"`
	APIVersion  string         `json:"apiVersion"`
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	TargetModel string         `json:"targetModel"`
	Overrides   map[string]any `json:"overrides"`
}

func ReadOverlay(path string) (Overlay, error) {
	file, err := os.Open(path)
	if err != nil {
		return Overlay{}, err
	}
	defer file.Close()

	return decodeOverlay(file, path)
}

// DecodeOverlay decodes and structurally validates an ODDC overlay from an
// already-open reader. It exists so privileged callers can read protected
// machine state without weakening the canonical overlay parser.
func DecodeOverlay(reader io.Reader) (Overlay, error) {
	return decodeOverlay(reader, "overlay")
}

func decodeOverlay(
	reader io.Reader,
	label string,
) (Overlay, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var overlay Overlay
	if err := decoder.Decode(&overlay); err != nil {
		return Overlay{}, fmt.Errorf(
			"decode overlay %s: %w",
			label,
			err,
		)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Overlay{}, fmt.Errorf(
				"%s has trailing JSON",
				label,
			)
		}
		return Overlay{}, err
	}

	if err := validateOverrideObject(
		overlay.Overrides,
		"overrides",
	); err != nil {
		return Overlay{}, fmt.Errorf(
			"%s: %w",
			label,
			err,
		)
	}

	return overlay, nil
}

func validateOverrideObject(value any, path string) error {
	switch typed := value.(type) {
	case []any:
		return fmt.Errorf(
			"%s is positional; overrides must use keyed objects",
			path,
		)

	case map[string]any:
		if marker, exists := typed["$delete"]; exists {
			deleted, ok := marker.(bool)
			if len(typed) != 1 || !ok || !deleted {
				return fmt.Errorf(
					"%s deletion marker must be exactly {\"$delete\": true}",
					path,
				)
			}

			return nil
		}

		for key, child := range typed {
			next := key
			if path != "" {
				next = path + "." + key
			}

			if err := validateOverrideObject(
				child,
				next,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func deleteMarker(value any) bool {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 1 {
		return false
	}

	deleted, ok := object["$delete"].(bool)
	return ok && deleted
}

func validateOverlay(
	overlay Overlay,
	modelID string,
	wantKind string,
) error {
	if overlay.APIVersion != EntityAPIVersion {
		return fmt.Errorf(
			"overlay %q apiVersion=%q want=%q",
			overlay.ID,
			overlay.APIVersion,
			EntityAPIVersion,
		)
	}

	if strings.TrimSpace(overlay.ID) == "" {
		return fmt.Errorf("overlay id is required")
	}

	if overlay.Kind != wantKind {
		return fmt.Errorf(
			"overlay %q kind=%q want=%q",
			overlay.ID,
			overlay.Kind,
			wantKind,
		)
	}

	if overlay.TargetModel != modelID {
		return fmt.Errorf(
			"overlay %q targets %q, resolving %q",
			overlay.ID,
			overlay.TargetModel,
			modelID,
		)
	}

	if overlay.Overrides == nil {
		return fmt.Errorf(
			"overlay %q has no overrides",
			overlay.ID,
		)
	}

	return validateOverrideObject(
		overlay.Overrides,
		"overrides",
	)
}
