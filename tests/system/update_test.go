package system_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/system"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// systemFlake writes a flake.nix that takes ODDC as the README shows.
func systemFlake(t *testing.T) (dir, readme string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(fixture.Repository, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	readme = strings.TrimSpace(regexp.MustCompile(`(?m)^ *inputs\.oddc\.url = .*$`).FindString(string(data)))
	if readme == "" {
		t.Fatal("README shows no oddc input")
	}

	dir = t.TempDir()
	fixture.Write(t, filepath.Join(dir, "flake.nix"), []byte("{\n  "+readme+"\n  outputs = _: { };\n}\n"))

	return dir, readme
}

func TestSetStage(t *testing.T) {
	flake, readme := systemFlake(t)
	read := func() string {
		data, err := os.ReadFile(filepath.Join(flake, "flake.nix"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	original := read()

	if err := system.SetStage(flake, system.Stages["staging"]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(), `"github:JadeOpenServices/oddc/staging"`) {
		t.Fatalf("not on staging:\n%s", read())
	}

	if err := system.SetStage(flake, system.Stages["main"]); err != nil {
		t.Fatal(err)
	}
	if read() != original {
		t.Fatalf("main is not the README's input:\n%s", read())
	}

	fixture.Write(t, filepath.Join(flake, "flake.nix"), []byte(original+"# "+readme+"\n"))
	if err := system.SetStage(flake, system.Stages["staging"]); err == nil {
		t.Fatal("chose between two ODDC URLs")
	}
}

// remoteHead is the commit a branch of ODDC on GitHub points at.
func remoteHead(t *testing.T, branch string) string {
	t.Helper()

	out, err := exec.Command("git", "ls-remote", "https://github.com/JadeOpenServices/oddc.git", "refs/heads/"+branch).Output()
	if err != nil || len(out) < 40 {
		t.Skipf("GitHub unreachable: %v", err)
	}

	return string(out[:40])
}

// Updating moves a real system flake between stages on GitHub. It needs
// nix and the network, so it skips without them, as in the Nix build.
func TestUpdateStage(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("no nix")
	}
	heads := map[string]string{"main": remoteHead(t, "main"), "staging": remoteHead(t, "staging")}

	flake, _ := systemFlake(t)
	if out, err := exec.Command("nix", "flake", "lock", flake).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	for _, stage := range []string{"main", "staging", "staging", "main"} {
		if err := system.RunUpdate([]string{"update", "--flake", flake, "--stage", stage}); err != nil {
			t.Fatal(err)
		}

		input, err := system.LockedOddc(flake)
		if err != nil {
			t.Fatal(err)
		}
		if input.Stage() != stage || input.Locked.Rev != heads[stage] {
			t.Fatalf("on %s %s, want %s %s", input.Stage(), input.Locked.Rev, stage, heads[stage])
		}
	}

	if err := system.RunUpdate([]string{"update", "--flake", flake, "--stage", "release"}); err == nil {
		t.Fatal("took an unknown stage")
	}
}
