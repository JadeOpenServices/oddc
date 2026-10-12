// SPDX-License-Identifier: GPL-3.0-or-later

package system_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// On every deployed model that references findings, users get each
// finding's summary and the way to the details, programs get the flags as
// false, and contributors get what, attack and fix.
func TestSecurityCommand(t *testing.T) {
	registry, models := fixture.Catalog(t)

	found := 0
	for _, model := range models {
		resolved, err := registry.ResolveModel(model, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		findings := oddc.Findings(resolved.Resolved)
		if len(findings) == 0 {
			continue
		}
		found++
		file := filepath.Join(fixture.Deployment(t, registry, model, false), "resolved.json")
		run := func(extra ...string) string {
			return fixture.Stdout(t, func() error {
				return system.RunSecurity(append([]string{"oddc", "security", "--resolved", file}, extra...))
			})
		}

		notice := run()
		details := run("--details")
		for _, f := range findings {
			if !strings.Contains(notice, f.Summary) {
				t.Errorf("%s: the notice lacks the summary of %s:\n%s", model, f.ID, notice)
			}
			for _, key := range []string{"what", "attack", "fix"} {
				if !strings.Contains(details, f.Details[key]) {
					t.Errorf("%s: the details lack %s of %s", model, key, f.ID)
				}
			}
		}
		if !strings.Contains(notice, "oddc security --details") || !strings.Contains(notice, "nothing to install") {
			t.Errorf("%s: the notice does not calm and point to the details:\n%s", model, notice)
		}

		var out struct {
			Flags map[string]bool `json:"flags"`
		}
		if err := json.Unmarshal([]byte(run("--json")), &out); err != nil {
			t.Fatal(err)
		}
		for _, f := range findings {
			for flag := range f.Flags {
				if value, named := out.Flags[flag]; !named || value {
					t.Errorf("%s: --json flag %s = %v (named %v), want false", model, flag, value, named)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no model references a security finding")
	}
}
