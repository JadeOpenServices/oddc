// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
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
			works = append(works, partEntry(data, key))
		case section == "hardware" && value == "unsupported":
			reason := unsupported[key].Reason
			broken = append(broken, fmt.Sprintf("%s, %s", partEntry(data, key), reason))
		case section == "capabilities" && value == "pass":
			works = append(works, capitalize(words(rest)))
		case section == "capabilities" && value == "not-exposed":
			absent = append(absent, capitalize(words(rest)))
		case section == "policy" && value == "pass":
			// A policy section says what ODDC makes the machine do; its
			// summary is that in the owner's words.
			summary, _ := componentAt(data, key)["summary"].(string)
			if summary == "" {
				summary = words(rest) + " policy"
			}
			fixed = append(fixed, summary)
		case section == "quirks" && value == "pass":
			// A quirk's name says what was wrong; its summary says what the
			// owner gets.
			text, _ := componentAt(data, key)["summary"].(string)
			if text == "" {
				text, _ = componentAt(data, key)["name"].(string)
			}
			if text == "" {
				text = words(rest)
			}
			fixed = append(fixed, text)
		}
	}
	list(&b, "Works", works)
	list(&b, "Works because of ODDC", fixed)
	list(&b, "Does not work yet", broken)
	list(&b, "Not built into this machine", absent)

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

// partEntry names a part for its owner: what it is for, from where the
// model places it, then the part itself: "Card reader: Realtek RTS522A".
func partEntry(data map[string]any, path string) string {
	label := partLabel(path)
	if name, _ := componentAt(data, path)["name"].(string); name != "" {
		return label + ": " + name
	}
	return label
}

// capitalize upper-cases the first letter.
func capitalize(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(first)) + text[size:]
}

// partLabel is what a part is for, read from the path the model places it
// at: "hardware.storage.cardReader.primary" is the card reader,
// "hardware.camera.front" the front camera. The path is the function; the
// component's name is the chip.
func partLabel(path string) string {
	segments := strings.Split(strings.TrimPrefix(path, "hardware."), ".")
	// "primary" only numbers a part.
	if n := len(segments); n > 1 && segments[n-1] == "primary" {
		segments = segments[:n-1]
	}
	// A grouping segment says nothing about the part itself.
	if len(segments) > 1 {
		switch segments[0] {
		case "security", "network", "input", "storage":
			segments = segments[1:]
		}
	}
	// A trailing qualifier reads better in front: "front camera".
	if n := len(segments); n > 1 {
		switch segments[n-1] {
		case "front", "rear", "internal", "external", "discrete", "integrated", "left", "right":
			segments = append([]string{segments[n-1]}, segments[:n-1]...)
		}
	}
	parts := make([]string, len(segments))
	for i, segment := range segments {
		if spelled, ok := spellings[segment]; ok {
			parts[i] = spelled
		} else {
			parts[i] = words(segment)
		}
	}
	return capitalize(strings.Join(parts, " "))
}

// spellings are path words that are not written in lower case.
var spellings = map[string]string{
	"usb":  "USB",
	"usbC": "USB-C",
	"wifi": "Wi-Fi",
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
