package oddc

import (
	"os"
	"strings"
	"testing"
)

// Release tags are only signed with SSH ed25519 keys held on a security
// key, limited to git signatures. See docs/RELEASES.md.
func TestAllowedSignersHoldOnlySecurityKeys(t *testing.T) {
	data, err := os.ReadFile("keys/allowed_signers")
	if err != nil {
		t.Fatal(err)
	}

	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 ||
			!strings.Contains(fields[1], `namespaces="git"`) ||
			fields[2] != "sk-ssh-ed25519@openssh.com" {
			t.Errorf("line %d: want PRINCIPAL namespaces=\"git\" sk-ssh-ed25519@openssh.com KEY: %q", n+1, line)
		}
	}
}
