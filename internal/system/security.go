// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunSecurity prints the security findings the deployed model references
// (--resolved, default /etc/oddc/resolved.json). Without options it is the
// notice for users: each finding's summary and that nothing needs doing.
// --details is the full account for contributors: what, attack, severity,
// fix. --json is for programs: the findings and the combined flags, where a
// flag no finding names is absent and means unknown, never protected.
func RunSecurity(args []string) error {
	data, err := os.ReadFile(cli.Value(args, "--resolved", "/etc/oddc/resolved.json"))
	if err != nil {
		return err
	}
	var resolved map[string]any
	if err := json.Unmarshal(data, &resolved); err != nil {
		return err
	}
	var active []oddc.Finding
	for _, f := range oddc.Findings(resolved) {
		if f.Active() {
			active = append(active, f)
		}
	}

	switch {
	case cli.Has(args, "--json"):
		out := json.NewEncoder(os.Stdout)
		out.SetIndent("", "  ")
		if active == nil {
			active = []oddc.Finding{}
		}
		return out.Encode(map[string]any{"findings": active, "flags": oddc.SecurityFlags(active)})
	case cli.Has(args, "--details"):
		for i, f := range active {
			if i > 0 {
				fmt.Println()
			}
			printDetails(f)
		}
		if len(active) == 0 {
			fmt.Println("No known security findings for this model.")
		}
	default:
		fmt.Print(SecurityNotice(active))
	}
	return nil
}

// SecurityNotice is the user notice for active findings, empty-handed when
// there are none.
func SecurityNotice(active []oddc.Finding) string {
	if len(active) == 0 {
		return "No known security findings for this model.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Known security findings for this model: %d. The ODDC team is on them; nothing for you to do.\n", len(active))
	for _, f := range active {
		fmt.Fprintf(&b, "- %s: %s\n", f.Name, f.Summary)
	}
	b.WriteString("Contributors: `oddc security --details` shows the full account.\n")
	return b.String()
}

func printDetails(f oddc.Finding) {
	fmt.Printf("%s (%s)\n", f.Name, f.ID)
	fmt.Printf("  severity %s, %s; reported %s, updated %s; referenced at %s\n", f.Severity, f.Status, f.Reported, f.Modified, f.Path)
	for _, key := range []string{"what", "attack", "fix", "when"} {
		if text := f.Details[key]; text != "" {
			fmt.Printf("  %s: %s\n", key, text)
		}
	}
	var rest []string
	for key := range f.Details {
		switch key {
		case "what", "attack", "fix", "when":
		default:
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		fmt.Printf("  %s: %s\n", key, f.Details[key])
	}
	var flags []string
	for key, value := range f.Flags {
		flags = append(flags, fmt.Sprintf("%s=%v", key, value))
	}
	sort.Strings(flags)
	if len(flags) > 0 {
		fmt.Printf("  flags: %s\n", strings.Join(flags, " "))
	}
}
