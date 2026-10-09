// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// revision is the recorded revision of a fetched answer or deployment,
// else the commit checked out at root, else "local".
func revision(root string) string {
	return oddc.DirSource{Root: root}.Revision()
}

// evidenceChanges lists evidence files that were changed or removed since
// the commit where root's HEAD branched from since. Evidence is append-only:
// only added files are allowed.
func evidenceChanges(root, since string) ([]string, error) {
	base, err := exec.Command("git", "-C", root, "merge-base", since, "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("--since %s: no common commit with HEAD: %w", since, err)
	}

	out, err := exec.Command(
		"git", "-C", root, "diff", "--name-status", "--no-renames",
		strings.TrimSpace(string(base)), "--", "evidence",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("--since %s: %w", since, err)
	}

	var changes []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		status, file, ok := strings.Cut(line, "\t")
		if !ok || status == "A" {
			continue
		}

		verb := "changed"
		if status == "D" {
			verb = "removed"
		}
		changes = append(changes, fmt.Sprintf(
			"%s %s since %s; evidence is append-only",
			file,
			verb,
			since,
		))
	}

	return changes, nil
}

// RunValidate fails when the catalog is invalid or, with since, when
// evidence was changed or removed since that revision; with --json it
// prints the full result either way.
func RunValidate(root, since string, asJSON bool) error {
	result := oddc.Validate(root)
	result.Revision = revision(root)

	if since != "" {
		changes, err := evidenceChanges(root, since)
		if err != nil {
			changes = []string{err.Error()}
		}
		if len(changes) > 0 {
			result.Valid = false
			result.Errors = append(result.Errors, changes...)
		}
	}

	if asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}

	if !result.Valid {
		return errors.New(strings.Join(result.Errors, "; "))
	}

	if !asJSON {
		fmt.Printf(
			"PASS: ODDC v2 entity registry valid (%d entities, %d evidence)\n",
			result.Entities,
			result.Evidence,
		)
	}

	return nil
}
