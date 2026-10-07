package oddc

import (
	"path/filepath"
	"testing"
)

// Each case adds one identifying detail to a copy of every real evidence
// record and expects validation to refuse it.
func TestBrokenCatalogIdentifyingEvidence(t *testing.T) {
	cases := map[string]func(record map[string]any){
		"serial number key": func(r map[string]any) { environment(r)["serialNumber"] = "FRANBMCP0A1234" },
		"hostname key":      func(r map[string]any) { environment(r)["host_name"] = "laptop" },
		"user key":          func(r map[string]any) { environment(r)["User"] = "jade" },
		"nested key":        func(r map[string]any) { environment(r)["network"] = map[string]any{"mac": "x"} },
		"MAC address":       func(r map[string]any) { environment(r)["wifi"] = "a8:6d:aa:01:02:03" },
		"UUID":              func(r map[string]any) { environment(r)["board"] = "4c4c4544-0042-3510-8052-b4c04f4e3732" },
		"machine ID":        func(r map[string]any) { environment(r)["boot"] = "0123456789abcdef0123456789abcdef" },
		"e-mail address":    func(r map[string]any) { environment(r)["tester"] = "someone@example.org" },
		"IPv4 address":      func(r map[string]any) { environment(r)["gateway"] = "192.168.1.20" },
		"IPv6 address":      func(r map[string]any) { environment(r)["route"] = "fe80::1c2b:3aff:fe4d" },
		"home directory":    func(r map[string]any) { environment(r)["config"] = "/home/jade/.config" },
		"time of day":       func(r map[string]any) { r["observedAt"] = "2026-09-15T21:04:11Z" },
		"free-text result":  func(r map[string]any) { r["results"].(map[string]any)["wifi"] = "works at Jade's place" },
		"non-string result": func(r map[string]any) { r["results"].(map[string]any)["wifi"] = true },
	}

	for what, edit := range cases {
		for _, path := range evidenceFiles(t) {
			root := copyCatalog(t)
			editEntity(t, filepath.Join(root, path), edit)
			expectRefused(t, root, what)
		}
	}
}

// Ordinary environment details stay allowed.
func TestEvidenceAllowsPlainEnvironment(t *testing.T) {
	for _, path := range evidenceFiles(t) {
		root := copyCatalog(t)
		editEntity(t, filepath.Join(root, path), func(r map[string]any) {
			environment(r)["os"] = "NixOS 25.11"
			environment(r)["kernel"] = "6.12.9"
			environment(r)["osName"] = "nixos"
			environment(r)["firmware"] = "3.05"
		})

		if result := Validate(root); !result.Valid {
			t.Errorf("%s: %v", path, result.Errors)
		}
	}
}

func environment(record map[string]any) map[string]any {
	env, ok := record["environment"].(map[string]any)
	if !ok {
		env = map[string]any{}
		record["environment"] = env
	}

	return env
}
