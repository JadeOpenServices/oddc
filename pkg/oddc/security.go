// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Security entities (kind "Security", catalog/entities/security/...) own
// what is known about a security weakness of hardware or its firmware. A
// model or component references them under its own "security" section; it
// never holds that knowledge itself. A finding only ever lowers trust: its
// flags are false, it takes effect wherever it is referenced, and a model
// that references it needs new evidence that the finding holds there.

// Severities of a finding, lowest first.
var securitySeverities = []string{"low", "medium", "high", "critical"}

// Statuses of a finding: open (nothing fixes it yet), mitigated (a fix
// exists, the details say which and where), withdrawn (it turned out not
// to hold; kept so references and history stay readable).
var securityStatuses = []string{"open", "mitigated", "withdrawn"}

// summaryLimit keeps the user notice short: one or two plain sentences.
const summaryLimit = 240

// Finding is a security entity as a resolved model references it.
type Finding struct {
	// Path is where the model holds the reference, as "security.thunderboltDma"
	// or "hardware.security.fingerprint.primary.security.matchOnHost".
	Path     string            `json:"path"`
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Severity string            `json:"severity"`
	Status   string            `json:"status"`
	Reported string            `json:"reported"`
	Modified string            `json:"modified"`
	Summary  string            `json:"summary"`
	Details  map[string]string `json:"details"`
	Flags    map[string]bool   `json:"flags"`
	// FwupdHsi are fwupd host security attributes whose failure confirms the
	// finding, so a test round can check it with fwupd and no device code.
	FwupdHsi []string `json:"fwupdHsi,omitempty"`
}

// Findings lists every security entity a resolved model references, at
// any depth, sorted by path. A reference is known by its id: only Security
// entities live below security/.
func Findings(resolved map[string]any) []Finding {
	var found []Finding
	var walk func(path string, value any)
	walk = func(path string, value any) {
		object, ok := value.(map[string]any)
		if !ok {
			return
		}
		if id, _ := object["id"].(string); strings.HasPrefix(id, "security/") {
			found = append(found, findingFrom(path, object))
			return
		}
		for key, child := range object {
			next := key
			if path != "" {
				next = path + "." + key
			}
			walk(next, child)
		}
	}
	walk("", resolved)
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found
}

func findingFrom(path string, object map[string]any) Finding {
	text := func(key string) string { s, _ := object[key].(string); return s }
	f := Finding{
		Path: path, ID: text("id"), Name: text("name"), Severity: text("severity"),
		Status: text("status"), Reported: text("reported"), Modified: text("modified"),
		Summary: text("summary"), Details: map[string]string{}, Flags: map[string]bool{},
	}
	details, _ := object["details"].(map[string]any)
	for key, value := range details {
		f.Details[key], _ = value.(string)
	}
	flags, _ := object["flags"].(map[string]any)
	for key, value := range flags {
		f.Flags[key], _ = value.(bool)
	}
	hsi, _ := object["fwupdHsi"].(map[string]any)
	for _, value := range hsi {
		if text, ok := value.(string); ok {
			f.FwupdHsi = append(f.FwupdHsi, text)
		}
	}
	sort.Strings(f.FwupdHsi)
	return f
}

// Active is false for a withdrawn finding, which no longer counts.
func (f Finding) Active() bool { return f.Status != "withdrawn" }

// SecurityFlags combines the flags of the active findings. A flag is
// false as soon as one finding says so. A flag no finding names is not in
// the map: a consumer treats it as unknown, never as protected.
func SecurityFlags(findings []Finding) map[string]bool {
	flags := map[string]bool{}
	for _, f := range findings {
		if !f.Active() {
			continue
		}
		for key, value := range f.Flags {
			if old, seen := flags[key]; seen {
				value = old && value
			}
			flags[key] = value
		}
	}
	return flags
}

// securityErrors checks every Security entity: a short summary for the user
// notice, the full account in details, known severity and status, dates,
// and flags that only lower trust.
func (r *Registry) securityErrors() []string {
	var errs []string
	ids := make([]string, 0, len(r.Entities))
	for id := range r.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		entity := r.Entities[id]
		below := strings.HasPrefix(id, "security/")
		switch {
		case entity.Kind != "Security" && !below:
			continue
		case entity.Kind != "Security":
			errs = append(errs, fmt.Sprintf("%s: below security/ only Security entities live, not %s", id, entity.Kind))
			continue
		case !below:
			errs = append(errs, fmt.Sprintf("%s: a Security entity lives below security/", id))
		}
		f := findingFrom(id, entity.Data)
		switch {
		case f.Summary == "":
			errs = append(errs, fmt.Sprintf("%s: data.summary is missing: one or two plain sentences for the user notice", id))
		case len(f.Summary) > summaryLimit:
			errs = append(errs, fmt.Sprintf("%s: data.summary has %d characters, at most %d", id, len(f.Summary), summaryLimit))
		}
		for _, key := range []string{"what", "attack", "fix"} {
			if f.Details[key] == "" {
				errs = append(errs, fmt.Sprintf("%s: data.details.%s is missing", id, key))
			}
		}
		if !slices.Contains(securitySeverities, f.Severity) {
			errs = append(errs, fmt.Sprintf("%s: data.severity %q is not one of %s", id, f.Severity, strings.Join(securitySeverities, ", ")))
		}
		if !slices.Contains(securityStatuses, f.Status) {
			errs = append(errs, fmt.Sprintf("%s: data.status %q is not one of %s", id, f.Status, strings.Join(securityStatuses, ", ")))
		}
		if f.Reported == "" || f.Modified == "" || f.Modified < f.Reported {
			errs = append(errs, fmt.Sprintf("%s: data.reported and data.modified are dates, modified not before reported", id))
		}
		if raw, present := entity.Data["fwupdHsi"]; present {
			named, _ := raw.(map[string]any)
			if len(named) == 0 {
				errs = append(errs, fmt.Sprintf("%s: data.fwupdHsi names fwupd HSI attributes, each under a name of its own", id))
			}
			for _, value := range named {
				if text, _ := value.(string); !strings.HasPrefix(text, "org.fwupd.hsi.") {
					errs = append(errs, fmt.Sprintf("%s: data.fwupdHsi %v is not an org.fwupd.hsi. attribute", id, value))
				}
			}
		}
		flags, _ := entity.Data["flags"].(map[string]any)
		for key, value := range flags {
			if value != false {
				errs = append(errs, fmt.Sprintf("%s: data.flags.%s is %v; a finding only lowers trust, so its flags are false", id, key, value))
			}
		}
	}
	return errs
}
