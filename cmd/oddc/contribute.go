package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/JadeOpenServices/oddc"
)

// Contributor commands. A contribution is data: files below catalog/ and
// evidence/ of a workspace checkout, sent as one pull request against
// staging from the contributor's own GitHub account.

const (
	upstreamGit = "https://github.com/" + oddc.Repository + ".git"
	baseBranch  = "staging"
)

// gh runs the GitHub CLI, signed in as the contributor.
func gh(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh %s: %v: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(string(out)), nil
}

func git(dir string, args ...string) (string, error) {
	return gitEnv(dir, nil, args...)
}

func gitEnv(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(string(out)), nil
}

func nulSeparated(out string) []string {
	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// runWorkspace clones the catalog on staging to the workspace, or updates
// an existing workspace to the newest staging.
func runWorkspace(args []string) error {
	dir := value(args, "--root", workspaceDir())

	if !exists(filepath.Join(dir, ".git")) {
		if err := command(
			"git", "clone", "--quiet", "--branch", baseBranch,
			value(args, "--from", upstreamGit), dir,
		); err != nil {
			return err
		}

		fmt.Printf("Workspace %s is on %s.\n", dir, baseBranch)
		return nil
	}

	if _, err := git(dir, "fetch", "--quiet", "origin", baseBranch); err != nil {
		return err
	}

	branch, _ := git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branch != baseBranch {
		return fmt.Errorf(
			"workspace %s is not on %s; fetched it without updating",
			dir,
			baseBranch,
		)
	}

	upstream := "origin/" + baseBranch
	if err := dropMerged(dir, upstream); err != nil {
		return err
	}

	if _, err := git(dir, "merge", "--quiet", "--ff-only", upstream); err != nil {
		return err
	}

	head, err := git(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return err
	}

	fmt.Printf("Workspace %s is on %s at %s.\n", dir, baseBranch, head)
	return nil
}

// dropMerged undoes local changes that upstream now holds with the same
// content, such as a contribution that was merged, so updating can
// fast-forward over them.
func dropMerged(dir, upstream string) error {
	out, err := git(dir, "ls-files", "-z", "--modified", "--others", "--exclude-standard")
	if err != nil {
		return err
	}

	for _, path := range nulSeparated(out) {
		local, err := git(dir, "hash-object", "--", path)
		if err != nil {
			continue
		}

		merged, err := git(dir, "rev-parse", "--verify", "--quiet", upstream+":"+path)
		if err != nil || merged != local {
			continue
		}

		if _, err := git(dir, "cat-file", "-e", "HEAD:"+path); err == nil {
			_, err = git(dir, "checkout", "--", path)
			if err != nil {
				return err
			}
			continue
		}

		if err := os.Remove(filepath.Join(dir, path)); err != nil {
			return err
		}
	}

	return nil
}

// readFacts reads facts from --facts FILE, else the sysfs below --sys.
func readFacts(args []string) (oddc.Facts, error) {
	path := value(args, "--facts", "")
	if path == "" {
		return oddc.ReadFacts(value(args, "--sys", "/sys")), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return oddc.Facts{}, err
	}

	var facts oddc.Facts
	if err := json.Unmarshal(data, &facts); err != nil {
		return oddc.Facts{}, fmt.Errorf("%s: %w", path, err)
	}

	return facts, nil
}

// loadWorkspace loads the catalog of --root, by default the workspace.
func loadWorkspace(args []string) (string, *oddc.Registry, error) {
	root := value(args, "--root", workspaceDir())
	if !exists(filepath.Join(root, "catalog")) {
		return "", nil, fmt.Errorf("%s holds no catalog; run `oddc workspace` first", root)
	}

	registry, err := oddc.LoadRegistry(root)
	return root, registry, err
}

// schemaRef is the relative $schema of a file below root.
func schemaRef(root, file, schema string) string {
	relative, err := filepath.Rel(filepath.Dir(file), filepath.Join(root, "schemas", schema))
	if err != nil {
		return ""
	}

	return filepath.ToSlash(relative)
}

// writeChecked writes a new file and keeps it only when the catalog
// still validates with it.
func writeChecked(root, file string, document any) error {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = out.Write(append(data, '\n'))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}

	if err == nil {
		if result := oddc.Validate(root); !result.Valid {
			err = errors.New(strings.Join(result.Errors, "; "))
		}
	}

	if err != nil {
		os.Remove(file)
		return fmt.Errorf("not written: %w", err)
	}

	return nil
}

// runScaffold drafts a model entity for this machine in the workspace.
func runScaffold(args []string) error {
	root, registry, err := loadWorkspace(args)
	if err != nil {
		return err
	}

	facts, err := readFacts(args)
	if err != nil {
		return err
	}

	classification, err := registry.Classify(facts)
	if err != nil {
		return err
	}
	switch classification.Result {
	case oddc.ResultMatched:
		return fmt.Errorf(
			"this machine already matches %s; record evidence with `oddc evidence record`",
			classification.Model,
		)
	case oddc.ResultAmbiguous:
		return fmt.Errorf(
			"this machine already matches %s",
			strings.Join(classification.Ambiguous, ", "),
		)
	}

	entity, notes, err := registry.DraftModel(value(args, "--id", ""), facts)
	if err != nil {
		return err
	}

	file := oddc.EntityPath(filepath.Join(root, "catalog", "entities"), entity.Metadata.ID)
	entity.Schema = schemaRef(root, file, "entity.schema.json")
	if err := writeChecked(root, file, entity); err != nil {
		return err
	}

	fmt.Printf("Drafted %s in %s.\n", entity.Metadata.ID, file)
	if len(notes) > 0 {
		fmt.Println("Present but not in the draft:")
		for _, note := range notes {
			fmt.Println("  " + note)
		}
	}
	fmt.Println("Review it, then record evidence with `oddc evidence record`.")

	return nil
}

var (
	evidenceStatuses = []string{
		"documented", "detected", "configured", "runtime-verified", "hardware-validated",
	}

	// kernelVersion keeps the upstream version of a kernel release and
	// drops a local suffix, which can name a machine.
	kernelVersion = regexp.MustCompile(`^\d+\.\d+(?:\.\d+)?`)
)

// osName is NAME and VERSION_ID from os-release, such as "NixOS 25.11".
func osName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}

	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			fields[key] = strings.Trim(value, `"'`)
		}
	}

	return strings.TrimSpace(fields["NAME"] + " " + fields["VERSION_ID"])
}

func kernelRelease() string {
	data, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return kernelVersion.FindString(strings.TrimSpace(string(data)))
}

// runEvidence records a new evidence file for this machine's model, or
// --device, in the workspace. Records are only ever added.
func runEvidence(args []string) error {
	if len(args) < 2 || args[1] != "record" {
		return errors.New("usage: oddc evidence record [--device ID] [--result NAME=STATUS]... [--status STATUS]")
	}

	root, registry, err := loadWorkspace(args)
	if err != nil {
		return err
	}

	facts, err := readFacts(args)
	if err != nil {
		return err
	}

	classification, err := registry.Classify(facts)
	if err != nil {
		return err
	}

	device := value(args, "--device", classification.Model)
	if device == "" {
		return errors.New("no model matches this machine; name it with --device, or draft one with `oddc scaffold`")
	}

	results := map[string]any{}
	if classification.Result == oddc.ResultMatched && classification.Model == device {
		results["identity"] = "pass"
	}
	for _, result := range values(args, "--result") {
		name, status, ok := strings.Cut(result, "=")
		if !ok {
			return fmt.Errorf("--result %q is not NAME=STATUS", result)
		}
		results[name] = status
	}
	if len(results) == 0 {
		return errors.New("nothing to record; add --result NAME=STATUS")
	}

	status := value(args, "--status", "detected")
	if !slices.Contains(evidenceStatuses, status) {
		return fmt.Errorf("--status %q is not one of %s", status, strings.Join(evidenceStatuses, ", "))
	}

	environment := map[string]any{}
	if name := value(args, "--os", osName()); name != "" {
		environment["os"] = name
	}
	if kernel := value(args, "--kernel", kernelRelease()); kernel != "" {
		environment["kernel"] = kernel
	}

	date := value(args, "--date", time.Now().UTC().Format(time.DateOnly))
	dir := filepath.Join(root, "evidence", filepath.FromSlash(device))
	name := date
	for n := 2; exists(filepath.Join(dir, name+".json")); n++ {
		name = fmt.Sprintf("%s-%d", date, n)
	}
	file := filepath.Join(dir, name+".json")

	record := oddc.Evidence{
		Schema:        schemaRef(root, file, "evidence.schema.json"),
		SchemaVersion: oddc.SchemaVersion,
		ID:            path.Base(device) + "-" + name,
		DeviceID:      device,
		ObservedAt:    date,
		Status:        status,
		Environment:   environment,
		Results:       results,
	}
	if err := writeChecked(root, file, record); err != nil {
		return err
	}

	fmt.Printf("Recorded %s.\nSend it with `oddc contribute`.\n", file)
	return nil
}

// contribution lists the workspace files that differ from base. It
// refuses files outside catalog/ and evidence/.
func contribution(root, base string) ([]string, error) {
	changed, err := git(root, "diff", "-z", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}

	untracked, err := git(root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	paths := append(nulSeparated(changed), nulSeparated(untracked)...)
	sort.Strings(paths)
	paths = slices.Compact(paths)

	var outside []string
	for _, path := range paths {
		if !strings.HasPrefix(path, "catalog/") && !strings.HasPrefix(path, "evidence/") {
			outside = append(outside, path)
		}
	}
	if len(outside) > 0 {
		return nil, fmt.Errorf(
			"contribute only sends catalog/ and evidence/; commit these yourself or remove them: %s",
			strings.Join(outside, ", "),
		)
	}

	return paths, nil
}

// pullRequestTitle names what a contribution adds.
func pullRequestTitle(root, base string, paths []string) string {
	var models, evidence []string
	for _, path := range paths {
		if _, err := git(root, "cat-file", "-e", base+":"+path); err == nil {
			continue
		}

		switch {
		case strings.HasPrefix(path, "catalog/entities/model/"):
			models = append(models, strings.TrimSuffix(strings.TrimPrefix(path, "catalog/entities/"), ".json"))
		case strings.HasPrefix(path, "evidence/"):
			evidence = append(evidence, filepath.ToSlash(filepath.Dir(strings.TrimPrefix(path, "evidence/"))))
		}
	}

	switch {
	case len(models) > 0:
		return "Add " + strings.Join(models, ", ")
	case len(evidence) > 0:
		return "Evidence for " + strings.Join(slices.Compact(evidence), ", ")
	default:
		return fmt.Sprintf("Update %d catalog files", len(paths))
	}
}

// prepared is a checked contribution: the tree it sends, built on the
// newest staging.
type prepared struct {
	base, tree, branch, title, body string
	paths                           []string
}

// prepareContribution checks the workspace's changes and builds their
// tree on the newest staging in a separate index, so the workspace itself
// stays as it is. Nothing leaves the machine unless the catalog validates
// and evidence was only added.
func prepareContribution(args []string) (prepared, error) {
	root := value(args, "--root", workspaceDir())
	p := prepared{base: "origin/" + baseBranch}

	if _, err := git(root, "fetch", "--quiet", "origin", baseBranch); err != nil {
		return p, err
	}

	paths, err := contribution(root, p.base)
	if err != nil {
		return p, err
	}
	if len(paths) == 0 {
		return p, fmt.Errorf("nothing to contribute: catalog/ and evidence/ match %s", baseBranch)
	}
	p.paths = paths

	if err := runValidate(root, p.base, false); err != nil {
		return p, err
	}

	p.title = value(args, "--title", pullRequestTitle(root, p.base, paths))
	p.body = value(args, "--body", "Files:\n\n- "+strings.Join(paths, "\n- ")+"\n\nSent with `oddc contribute`.")

	index, err := os.MkdirTemp("", "oddc-contribute-")
	if err != nil {
		return p, err
	}
	defer os.RemoveAll(index)
	indexEnv := []string{"GIT_INDEX_FILE=" + filepath.Join(index, "index")}

	if _, err := gitEnv(root, indexEnv, "read-tree", p.base); err != nil {
		return p, err
	}
	if _, err := gitEnv(root, indexEnv, append([]string{"add", "--all", "--"}, paths...)...); err != nil {
		return p, err
	}
	if p.tree, err = gitEnv(root, indexEnv, "write-tree"); err != nil {
		return p, err
	}

	// The branch is named after the contributed tree, so a contribution
	// that was already sent is not pushed twice.
	p.branch = "contrib/" + p.tree[:12]

	return p, nil
}

// runContribute sends the workspace's checked changes as a pull request
// against staging from the contributor's GitHub account, through a fork
// when the account cannot push to the catalog. The commit's author is the
// account's noreply address and its dates are in UTC.
func runContribute(args []string) error {
	root := value(args, "--root", workspaceDir())

	p, err := prepareContribution(args)
	if err != nil {
		return err
	}

	user, err := gh("api", "user", "--jq", `.login + " " + (.id | tostring)`)
	if err != nil {
		return err
	}
	var login string
	var id int64
	if _, err := fmt.Sscan(user, &login, &id); err != nil {
		return fmt.Errorf("gh api user: %q: %w", user, err)
	}

	now := fmt.Sprintf("@%d +0000", time.Now().Unix())
	email := fmt.Sprintf("%d+%s@users.noreply.github.com", id, login)
	commit, err := gitEnv(root, []string{
		"GIT_AUTHOR_NAME=" + login, "GIT_AUTHOR_EMAIL=" + email, "GIT_AUTHOR_DATE=" + now,
		"GIT_COMMITTER_NAME=" + login, "GIT_COMMITTER_EMAIL=" + email, "GIT_COMMITTER_DATE=" + now,
	}, "commit-tree", p.tree, "-p", p.base, "-m", p.title+"\n\n"+p.body)
	if err != nil {
		return err
	}

	repo, head := oddc.Repository, p.branch
	push, err := gh("api", "repos/"+repo, "--jq", ".permissions.push")
	if err != nil {
		return err
	}
	if push != "true" {
		if _, err := gh("repo", "fork", repo, "--clone=false", "--remote=false"); err != nil {
			return err
		}
		repo, head = login+"/"+path.Base(oddc.Repository), login+":"+p.branch
	}

	remote := "https://github.com/" + repo + ".git"
	sent, err := git(root, "ls-remote", remote, "refs/heads/"+p.branch)
	if err != nil {
		return err
	}
	if sent == "" {
		if _, err := git(root, "push", "--quiet", remote, commit+":refs/heads/"+p.branch); err != nil {
			return err
		}
	}

	url, err := gh(
		"pr", "create", "--repo", oddc.Repository, "--base", baseBranch, "--head", head,
		"--title", p.title, "--body", p.body,
	)
	if err != nil {
		return err
	}

	fmt.Println(url)
	return nil
}
