package contribute

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// schemaRef is the relative $schema of a file below root.
func schemaRef(root, file, schema string) string {
	relative, err := filepath.Rel(filepath.Dir(file), filepath.Join(root, "schemas", schema))
	if err != nil {
		return ""
	}

	return filepath.ToSlash(relative)
}

// writeChecked writes a new file and keeps it only when the catalog
// still validates with it.
func writeChecked(root, file string, document any) error {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = out.Write(append(data, '\n'))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}

	if err == nil {
		if result := oddc.Validate(root); !result.Valid {
			err = errors.New(strings.Join(result.Errors, "; "))
		}
	}

	if err != nil {
		os.Remove(file)
		return fmt.Errorf("not written: %w", err)
	}

	return nil
}

// RunScaffold drafts a model entity for this machine in the workspace.
func RunScaffold(args []string) error {
	root, registry, err := loadWorkspace(args)
	if err != nil {
		return err
	}

	facts, err := cli.ReadFacts(args)
	if err != nil {
		return err
	}

	classification, err := registry.Classify(facts)
	if err != nil {
		return err
	}
	switch classification.Result {
	case oddc.ResultMatched:
		return fmt.Errorf(
			"this machine already matches %s; record evidence with `oddc evidence record`",
			classification.Model,
		)
	case oddc.ResultAmbiguous:
		return fmt.Errorf(
			"this machine already matches %s",
			strings.Join(classification.Ambiguous, ", "),
		)
	}

	entity, notes, err := registry.DraftModel(cli.Value(args, "--id", ""), facts)
	if err != nil {
		return err
	}

	file := oddc.EntityPath(filepath.Join(root, "catalog", "entities"), entity.Metadata.ID)
	entity.Schema = schemaRef(root, file, "entity.schema.json")
	if err := writeChecked(root, file, entity); err != nil {
		return err
	}

	fmt.Printf("Drafted %s in %s.\n", entity.Metadata.ID, file)
	if len(notes) > 0 {
		fmt.Println("Present but not in the draft:")
		for _, note := range notes {
			fmt.Println("  " + note)
		}
	}
	fmt.Println("Review it, then record evidence with `oddc evidence record`.")

	return nil
}
