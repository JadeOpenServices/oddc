// SPDX-License-Identifier: GPL-3.0-or-later

package contribute

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/cli"
)

// errNoGh explains how to contribute without the questions, for a run
// without a terminal or a contributor who answered no.
var errNoGh = errors.New(
	"contribute needs the GitHub CLI signed in to GitHub: " +
		"run `nix shell nixpkgs#gh`, then `gh auth login`, then `oddc contribute` again",
)

// gh is the GitHub CLI this contribution uses.
type gh struct {
	bin string
	// login is true when this run signed gh in.
	login bool
}

func (g gh) run(args ...string) (string, error) {
	return cli.Gh(g.bin, args...)
}

// git makes git sign in to GitHub with gh's login instead of any
// credential helper of its own, and never ask for a password.
func (g gh) git() []string {
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=credential.https://github.com.helper", "GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=credential.https://github.com.helper",
		"GIT_CONFIG_VALUE_1=!'" + g.bin + "' auth git-credential",
	}
}

// findGh returns gh from PATH, or offers to build it from nixpkgs for this
// run only, as `nix shell nixpkgs#gh` would; nothing is installed. When gh
// is not signed in, it offers `gh auth login`.
func findGh() (gh, error) {
	bin, err := exec.LookPath("gh")
	if err != nil {
		if !cli.Ask("The GitHub CLI (gh) is not installed. Use it from nixpkgs for this contribution only?") {
			return gh{}, errNoGh
		}

		cmd := exec.Command("nix", "build", "nixpkgs#gh", "--no-link", "--print-out-paths")
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return gh{}, fmt.Errorf("nix build nixpkgs#gh: %w", err)
		}
		bin = filepath.Join(strings.TrimSpace(string(out)), "bin", "gh")
	}

	g := gh{bin: bin}
	if exec.Command(bin, "auth", "status", "--hostname", "github.com").Run() == nil {
		return g, nil
	}

	if !cli.Ask("gh is not signed in to GitHub. Sign in now?") {
		return gh{}, errNoGh
	}
	if err := cli.Command(bin, "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web"); err != nil {
		return gh{}, fmt.Errorf("gh auth login: %w", err)
	}
	g.login = true

	return g, nil
}

// done offers to sign gh out again when this run signed it in.
func (g gh) done() {
	if !g.login || !cli.Ask("Sign gh out of GitHub again?") {
		return
	}
	if err := cli.Command(g.bin, "auth", "logout", "--hostname", "github.com"); err != nil {
		fmt.Fprintf(os.Stderr, "gh auth logout: %v\n", err)
	}
}
