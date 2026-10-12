// SPDX-License-Identifier: GPL-3.0-or-later

package contribute_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A candidate is released only when it holds something new, is valid, and
// every model is verified; each problem is named.
func TestReleaseCandidate(t *testing.T) {
	root := fixture.GitCatalog(t)

	// Nothing new since the last release.
	c, err := contribute.CheckCandidate(root, "HEAD", "base")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Problems) != 1 || !strings.Contains(c.Problems[0], "holds nothing") {
		t.Errorf("same commit as the release: problems %v", c.Problems)
	}

	// A merge on top: the candidate's models are exactly as verified as the
	// catalog says, and every unverified one is a problem.
	git(t, root, "checkout", "-q", "-b", "work")
	fixture.Write(t, filepath.Join(root, "catalog", "NOTE"), []byte("release test\n"))
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "a change")
	git(t, root, "checkout", "-q", "-")
	git(t, root, "merge", "-q", "--no-ff", "-m", "Merge a change", "work")

	c, err = contribute.CheckCandidate(root, "HEAD", "base")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := oddc.LoadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	unverified := 0
	for id, status := range c.Models {
		want, err := registry.Verify(id)
		if err != nil {
			t.Fatal(err)
		}
		if status != want.Status {
			t.Errorf("%s: candidate says %s, catalog %s", id, status, want.Status)
		}
		if status != oddc.Verified {
			unverified++
			if !strings.Contains(strings.Join(c.Problems, "\n"), id) {
				t.Errorf("%s is %s but no problem names it: %v", id, status, c.Problems)
			}
		}
	}
	if len(c.Models) == 0 || len(c.Problems) != unverified {
		t.Errorf("models %v, problems %v", c.Models, c.Problems)
	}
	if len(c.Merges) != 1 || c.Merges[0] != "Merge a change" || !strings.Contains(c.Body(), "- Merge a change") {
		t.Errorf("merges %v, body:\n%s", c.Merges, c.Body())
	}

	// A broken catalog is never a release.
	entity := oddc.EntityPath(filepath.Join(root, "catalog", "entities"), "security/thunderbolt/no-kernel-dma-protection")
	data, err := os.ReadFile(entity)
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, entity, []byte(strings.Replace(string(data), `"severity": "medium"`, `"severity": "scary"`, 1)))
	git(t, root, "commit", "-q", "-am", "break it")
	c, err = contribute.CheckCandidate(root, "HEAD", "base")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(c.Problems, "\n"), "invalid:") {
		t.Errorf("a broken catalog passed: %v", c.Problems)
	}
}

// The release notes hold every new device's entry, and each QR code reads
// back as the pull request's URL.
func TestReleaseNotesAndQRCodes(t *testing.T) {
	zbarimg, err := exec.LookPath("zbarimg")
	if _, qr := exec.LookPath("qrencode"); err != nil || qr != nil {
		t.Skip("qrencode and zbarimg are not on PATH; the Nix build provides them")
	}
	root := fixture.GitCatalog(t)
	// A release with an empty tree: every model of the catalog is new.
	empty, err := exec.Command("git", "-C", root, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit-tree", "4b825dc642cb6eb9a060e54bf8d69288fbee4904", "-m", "nothing released yet").Output()
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "tag", "empty", strings.TrimSpace(string(empty)))

	c, err := contribute.CheckCandidate(root, "HEAD", "empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Notes) == 0 {
		t.Fatal("against an empty release every model is new, but no notes")
	}
	dir := t.TempDir()
	url := "https://github.com/" + oddc.Repository + "/pull/1"
	if err := contribute.WriteNotes(dir, url, c); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(dir, "release-notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	for id, notes := range c.Notes {
		title := strings.SplitN(notes, "\n", 2)[0]
		if !strings.Contains(string(text), strings.TrimLeft(title, "# ")) {
			t.Errorf("release notes lack %s (%s)", id, title)
		}
		base := filepath.Join(dir, strings.ReplaceAll(strings.TrimPrefix(id, "model/"), "/", "-")+"-qr")
		for _, file := range []string{base + ".png", base + ".svg"} {
			if _, err := os.Stat(file); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		}
		out, err := exec.Command(zbarimg, "--quiet", "--raw", base+".png").Output()
		if err != nil || strings.TrimSpace(string(out)) != url {
			t.Errorf("%s: QR code reads %q (%v), want %s", id, out, err, url)
		}
	}
}
