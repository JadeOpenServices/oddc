// SPDX-License-Identifier: GPL-3.0-or-later

package contribute

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// Candidate is staging checked as a release: the commit, what stable lacks
// of it, and whether it may become a release.
type Candidate struct {
	Commit string
	// Date is the commit's date, as 2006-01-02.
	Date string
	// Merges are the subjects of staging's first-parent merges stable lacks.
	Merges []string
	// Models maps every model to its verification status.
	Models map[string]string
	// Problems say why the candidate may not be released; empty when it may.
	Problems []string
	// Notes are the device entries (oddc notes) of the models that are new
	// or whose closure changed since base.
	Notes map[string]string
}

// CheckCandidate checks ref (a commit of the git checkout at root) against
// base the way CI guards stable: the catalog at ref is valid, every model is
// verified on its closure, and ref holds something base does not.
func CheckCandidate(root, ref, base string) (Candidate, error) {
	commit, err := cli.Git(root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return Candidate{}, err
	}
	c := Candidate{Commit: commit, Models: map[string]string{}, Notes: map[string]string{}}
	if c.Date, err = cli.Git(root, "log", "-1", "--format=%cs", commit); err != nil {
		return Candidate{}, err
	}

	ahead, err := cli.Git(root, "rev-list", "--count", base+".."+commit)
	if err != nil {
		return Candidate{}, err
	}
	if ahead == "0" {
		c.Problems = append(c.Problems, fmt.Sprintf("%s holds nothing that %s lacks", ref, base))
		return c, nil
	}
	merges, err := cli.Git(root, "log", "--first-parent", "--merges", "--format=%s", base+".."+commit)
	if err != nil {
		return Candidate{}, err
	}
	for _, line := range strings.Split(merges, "\n") {
		if line != "" {
			c.Merges = append(c.Merges, line)
		}
	}

	dir, err := checkout(root, commit)
	if err != nil {
		return Candidate{}, err
	}
	defer os.RemoveAll(dir)

	validation := oddc.Validate(dir)
	for _, problem := range validation.Errors {
		c.Problems = append(c.Problems, "invalid: "+problem)
	}
	if !validation.Valid {
		return c, nil
	}
	registry, err := oddc.LoadRegistry(dir)
	if err != nil {
		return Candidate{}, err
	}
	for id, entity := range registry.Entities {
		if entity.Kind != "DeviceModel" {
			continue
		}
		status, err := registry.Verify(id)
		if err != nil {
			return Candidate{}, err
		}
		c.Models[id] = status.Status
		if status.Status != oddc.Verified {
			c.Problems = append(c.Problems, fmt.Sprintf("%s is %s, not verified", id, status.Status))
			continue
		}
		// Every verified model gets its owner page in the release, so a
		// release that only improved the wording still ships the new page
		// (a summary is not part of the closure, so "changed since base"
		// would miss it). The page is a snapshot, not a changelog.
		if c.Notes[id], err = registry.DeviceNotes(id); err != nil {
			return Candidate{}, err
		}
	}
	sort.Strings(c.Problems)
	return c, nil
}

// demote moves every markdown heading two levels down, to sit inside the
// pull request's own text.
func demote(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			lines[i] = "##" + line
		}
	}
	return strings.Join(lines, "\n")
}

// checkout writes the files of a commit to a new temporary directory.
func checkout(root, ref string) (string, error) {
	dir, err := os.MkdirTemp("", "oddc-release-")
	if err != nil {
		return "", err
	}
	archive := exec.Command("sh", "-c", `git -C "$1" archive "$2" | tar -x -C "$3"`, "sh", root, ref, dir)
	if out, err := archive.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git archive %s: %v: %s", ref, err, strings.TrimSpace(string(out)))
	}
	return dir, nil
}

// Body is the release pull request's description.
func (c Candidate) Body() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Promotes staging (%s) to stable.\n\nMerged since the last release:\n\n", c.Commit[:12])
	for _, merge := range c.Merges {
		fmt.Fprintf(&b, "- %s\n", merge)
	}
	b.WriteString("\nModels:\n\n")
	ids := make([]string, 0, len(c.Models))
	for id := range c.Models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "- %s: %s\n", id, c.Models[id])
	}
	b.WriteString("\nChecked by `oddc release`: the catalog is valid and every model is verified on its closure.\n")
	ids = ids[:0]
	for id := range c.Notes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "\n<details><summary>%s: what works, what does not yet, what is planned</summary>\n\n%s\n</details>\n", id, demote(c.Notes[id]))
	}
	return b.String()
}

var (
	prNumber = regexp.MustCompile(`/pull/(\d+)$`)
	mergedPR = regexp.MustCompile(`^Merge pull request #(\d+) `)
)

// RunRelease promotes staging to stable for a maintainer: it checks staging
// as CI does, opens the release pull request (or reuses the open one),
// waits for its checks, asks before merging, merges with a merge commit
// and prints the release commit. It never pushes to stable and stops when
// GitHub reports a bypassed rule. --root is a git checkout of the catalog
// (default the workspace); --check stops after checking staging. --notes
// DIR writes the release notes and, per new or changed device, its own
// page and a QR code (SVG and PNG) for the people who gave or lent the
// machine; the code leads to --notes-url (with {device} standing for
// vendor-model), else to the release pull request.
func RunRelease(args []string) error {
	root := cli.Value(args, "--root", cli.WorkspaceDir())
	page := cli.Value(args, "--notes-url", "")

	g, err := findGh()
	if err != nil {
		return err
	}
	defer g.done()

	remote := "https://github.com/" + oddc.Repository + ".git"
	if _, err := cli.GitEnv(root, g.git(), "fetch", "--quiet", remote,
		"+refs/heads/staging:refs/oddc-release/staging", "+refs/heads/stable:refs/oddc-release/stable"); err != nil {
		return err
	}
	c, err := CheckCandidate(root, "refs/oddc-release/staging", "refs/oddc-release/stable")
	if err != nil {
		return err
	}
	// "Merge pull request #N from ..." says little; name each by its title.
	for i, merge := range c.Merges {
		if m := mergedPR.FindStringSubmatch(merge); m != nil {
			if title, err := g.run("pr", "view", m[1], "--repo", oddc.Repository, "--json", "title", "--jq", ".title"); err == nil {
				c.Merges[i] = "#" + m[1] + " " + title
			}
		}
	}
	fmt.Printf("staging %s:\n", c.Commit[:12])
	for _, merge := range c.Merges {
		fmt.Printf("  %s\n", merge)
	}
	if len(c.Problems) > 0 {
		return fmt.Errorf("staging cannot be released:\n  %s", strings.Join(c.Problems, "\n  "))
	}
	fmt.Println("valid, every model verified")
	if cli.Has(args, "--check") {
		return nil
	}

	number, err := g.run("pr", "list", "--repo", oddc.Repository, "--base", "stable", "--head", "staging",
		"--state", "open", "--json", "number", "--jq", ".[0].number // empty")
	if err != nil {
		return err
	}
	if number == "" {
		url, err := g.run("pr", "create", "--repo", oddc.Repository, "--base", "stable", "--head", "staging",
			"--title", "Release: staging "+c.Commit[:12], "--body", c.Body())
		if err != nil {
			return err
		}
		match := prNumber.FindStringSubmatch(url)
		if match == nil {
			return fmt.Errorf("gh pr create printed %q, not a pull request URL", url)
		}
		number = match[1]
		fmt.Println("opened", url)
	} else {
		fmt.Printf("release pull request #%s is open already\n", number)
	}
	if dir := cli.Value(args, "--notes", ""); dir != "" {
		url := "https://github.com/" + oddc.Repository + "/pull/" + number
		if err := WriteNotes(dir, url, page, c); err != nil {
			return err
		}
	}

	head, err := g.run("pr", "view", number, "--repo", oddc.Repository, "--json", "headRefOid", "--jq", ".headRefOid")
	if err != nil {
		return err
	}
	if head != c.Commit {
		return fmt.Errorf("pull request #%s is at %s, but the checked staging is %s; run oddc release again", number, head, c.Commit)
	}

	fmt.Println("waiting for the checks of #" + number)
	// Checks of the staging commit from earlier pushes are listed at once,
	// while the pull request's own runs may still be queued. So wait until
	// GitHub itself calls the pull request mergeable, watching whatever
	// checks run meanwhile.
	for start := time.Now(); ; time.Sleep(10 * time.Second) {
		state, err := g.run("pr", "view", number, "--repo", oddc.Repository, "--json", "mergeStateStatus", "--jq", ".mergeStateStatus")
		if err != nil {
			return err
		}
		if state == "CLEAN" || state == "HAS_HOOKS" {
			break
		}
		pending, _ := g.run("pr", "checks", number, "--repo", oddc.Repository, "--json", "bucket", "--jq", `map(select(.bucket == "pending")) | length`)
		if pending != "" && pending != "0" {
			if err := cli.Command(g.bin, "pr", "checks", number, "--repo", oddc.Repository, "--watch", "--fail-fast"); err != nil {
				return fmt.Errorf("the checks of #%s did not pass: %w", number, err)
			}
			continue
		}
		if time.Since(start) > 5*time.Minute {
			return fmt.Errorf("#%s is %s with no check running; see its page for what stable's rules still require", number, state)
		}
	}

	if !cli.Ask(fmt.Sprintf("Merge #%s into stable? This is the release.", number)) {
		return errors.New("not merged; run oddc release again to continue")
	}
	out, err := exec.Command(g.bin, "pr", "merge", number, "--repo", oddc.Repository, "--merge",
		"--match-head-commit", c.Commit).CombinedOutput()
	if strings.Contains(strings.ToLower(string(out)), "bypass") {
		return fmt.Errorf("GitHub reports a bypassed rule; stop and tell the maintainers:\n%s", out)
	}
	if err != nil {
		// A server error can come after GitHub merged; trust the pull request.
		state, _ := g.run("pr", "view", number, "--repo", oddc.Repository, "--json", "state", "--jq", ".state")
		if state != "MERGED" {
			return fmt.Errorf("gh pr merge: %v: %s", err, strings.TrimSpace(string(out)))
		}
		fmt.Printf("GitHub answered with an error, but #%s is merged\n", number)
	}

	release, err := g.run("api", "repos/"+oddc.Repository+"/commits/stable", "--jq", ".sha")
	if err != nil {
		return err
	}
	fmt.Printf("released: stable is %s; consumers pin this commit\n", release)
	return nil
}

// WriteNotes writes dir/release-notes.md (for maintainers: the pull
// request url and its text) and, per device in the notes, the page for the
// machine's owner, dir/<vendor>-<model>.md, with dir/<vendor>-<model>-qr.svg
// and .png. The code leads to page, a URL in which {device} stands for
// vendor-model; empty, it leads to url.
func WriteNotes(dir, url, page string, c Candidate) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ids := make([]string, 0, len(c.Notes))
	for id := range c.Notes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	fmt.Fprintf(&b, "# ODDC release %s\n\nPull request: %s\n\n%s", c.Commit[:12], url, c.Body())
	if len(ids) > 0 {
		b.WriteString("\nDevice pages in this directory:\n\n")
		for _, id := range ids {
			fmt.Fprintf(&b, "- %s.md\n", DeviceSlug(id))
		}
	}
	file := filepath.Join(dir, "release-notes.md")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote", file)
	if len(ids) == 0 {
		return nil
	}

	qrencode, err := tool("qrencode", "qrencode")
	if err != nil {
		return err
	}
	for _, id := range ids {
		slug := DeviceSlug(id)
		file := filepath.Join(dir, slug+".md")
		if err := os.WriteFile(file, []byte(DevicePage(c.Notes[id], c.Commit, c.Date)), 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", file)

		target := url
		if page != "" {
			target = strings.ReplaceAll(page, "{device}", slug)
		}
		base := filepath.Join(dir, slug+"-qr")
		for _, kind := range []string{"SVG", "PNG"} {
			out := base + "." + strings.ToLower(kind)
			if err := cli.Command(qrencode, "-t", kind, "-s", "10", "-m", "2", "-o", out, target); err != nil {
				return fmt.Errorf("qrencode %s: %w", out, err)
			}
			fmt.Println("wrote", out)
		}
	}
	return nil
}

// DeviceSlug is a model's file name: "model/hp/zbook-x2-g4" is hp-zbook-x2-g4.
func DeviceSlug(id string) string {
	return strings.ReplaceAll(strings.TrimPrefix(id, "model/"), "/", "-")
}

// DevicePage is a device's notes as the page its owner reads: the title,
// the release it comes from, then the notes.
func DevicePage(notes, commit, date string) string {
	title, rest, _ := strings.Cut(notes, "\n")
	return fmt.Sprintf("%s\n\nODDC release %s, %s.\n%s", title, commit[:12], date, rest)
}

// tool finds name on PATH, else builds the nixpkgs attribute for this run
// only, as `nix shell nixpkgs#attr` would; nothing is installed.
func tool(name, attr string) (string, error) {
	if bin, err := exec.LookPath(name); err == nil {
		return bin, nil
	}
	cmd := exec.Command("nix", "build", "nixpkgs#"+attr, "--no-link", "--print-out-paths")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s is not installed and nix build nixpkgs#%s failed: %w", name, attr, err)
	}
	return filepath.Join(strings.TrimSpace(string(out)), "bin", name), nil
}
