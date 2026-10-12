// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// DeviceNotes is a model's entry for people who own the machine, such as
// the person who gave it for testing: what works, what works because of
// ODDC, what does not work yet, security findings, what is planned, tips,
// and how it was tested. Everything comes from the catalog and the model's
// newest passing evidence; nothing is written twice.
func (r *Registry) DeviceNotes(model string) (string, error) {
	resolved, err := r.ResolveEntity(model)
	if err != nil {
		return "", err
	}
	data := resolved.Resolved
	status, err := r.Verify(model)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s on Linux\n\n", resolved.Name)

	record := status.Evidence
	results := map[string]any{}
	if record != nil {
		results = record.Results
	}
	switch status.Status {
	case Verified:
		fmt.Fprintf(&b, "Proven on the machine itself (%s, %s).\n", record.Status, record.ObservedAt)
	case Changed:
		fmt.Fprintf(&b, "Proven on the machine (%s, %s); the catalog changed since and is being re-tested.\n", record.Status, record.ObservedAt)
	default:
		b.WriteString("Not proven on the machine yet.\n")
	}

	var works, broken, fixed, absent []string
	unsupported := map[string]Unsupported{}
	for _, part := range UnsupportedComponents(data) {
		unsupported[part.Path] = part
	}
	keys := make([]string, 0, len(results))
	for key := range results {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := results[key]
		section, rest, _ := strings.Cut(key, ".")
		switch {
		case section == "hardware" && value == "pass":
			works = append(works, partName(data, key))
		case section == "hardware" && value == "unsupported":
			reason := unsupported[key].Reason
			broken = append(broken, fmt.Sprintf("%s: %s", partName(data, key), reason))
		case section == "capabilities" && value == "pass":
			works = append(works, words(rest))
		case section == "capabilities" && value == "not-exposed":
			absent = append(absent, words(rest))
		case section == "quirks" && value == "pass":
			name, _ := componentAt(data, key)["name"].(string)
			if name == "" {
				name = words(rest)
			}
			fixed = append(fixed, name)
		}
	}
	list(&b, "Works", works)
	list(&b, "Works because of ODDC", fixed)
	list(&b, "Does not work yet", broken)
	list(&b, "Not on this machine", absent)

	var security []string
	for _, f := range Findings(data) {
		if f.Active() {
			security = append(security, fmt.Sprintf("%s: %s", f.Name, f.Summary))
		}
	}
	list(&b, "Security", security)
	list(&b, "Planned", supportEntries(data, "support.planned"))
	list(&b, "Good to know", supportEntries(data, "support.notes"))

	if record != nil {
		b.WriteString("\n## How this was tested\n\n")
		fmt.Fprintf(&b, "Evidence %s (%s, %s)", record.ID, record.Status, record.ObservedAt)
		var env []string
		for _, key := range []string{"bios", "kernel", "os", "oddc"} {
			if value, _ := record.Environment[key].(string); value != "" {
				env = append(env, key+" "+value)
			}
		}
		if len(env) > 0 {
			fmt.Fprintf(&b, ": %s", strings.Join(env, ", "))
		}
		b.WriteString(".\n")
	}
	return b.String(), nil
}

// supportEntries lists the summaries below a support section, each with
// its work item when one is named.
func supportEntries(data map[string]any, path string) []string {
	var out []string
	section := componentAt(data, path)
	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry, _ := section[key].(map[string]any)
		summary, _ := entry["summary"].(string)
		if item, _ := entry["item"].(string); item != "" {
			summary += " (" + item + ")"
		}
		out = append(out, summary)
	}
	return out
}

// supportErrors checks every model's support.planned and support.notes:
// each entry has a summary.
func (r *Registry) supportErrors() []string {
	var errs []string
	for id, entity := range r.Entities {
		if entity.Kind != "DeviceModel" {
			continue
		}
		for _, path := range []string{"support.planned", "support.notes"} {
			for key, value := range componentAt(entity.Data, path) {
				entry, _ := value.(map[string]any)
				if summary, _ := entry["summary"].(string); strings.TrimSpace(summary) == "" {
					errs = append(errs, fmt.Sprintf("%s: %s.%s needs a summary", id, path, key))
				}
			}
		}
	}
	sort.Strings(errs)
	return errs
}

func list(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n", title)
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
}

// partName is a component's name, else its path in words.
func partName(data map[string]any, path string) string {
	if name, _ := componentAt(data, path)["name"].(string); name != "" {
		return name
	}
	return words(strings.TrimPrefix(path, "hardware."))
}

// words turns "kickstandSwitch" into "kickstand switch".
func words(key string) string {
	var b strings.Builder
	for i, r := range strings.ReplaceAll(key, ".", " ") {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
