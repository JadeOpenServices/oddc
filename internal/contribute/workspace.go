// SPDX-License-Identifier: GPL-3.0-or-later

// Package contribute holds the contributor commands. A contribution is
// data: files below catalog/ and evidence/ of a workspace checkout, sent as
// one pull request against staging, or a model's verify branch, from the
// contributor's own GitHub account.
package contribute

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

const (
	upstreamGit = "https://github.com/" + oddc.Repository + ".git"
	BaseBranch  = "staging"
)

// verifyBranch is the branch a model change is proven on before it
// reaches staging: verify/<vendor>/<model>.
var verifyBranch = regexp.MustCompile(`^verify/[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*$`)

// workspaceBranch is the upstream branch the workspace at dir follows: its
// verify branch when it is on one, else staging.
func workspaceBranch(dir string) string {
	branch, _ := cli.Git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if verifyBranch.MatchString(branch) {
		return branch
	}

	return BaseBranch
}

// RunWorkspace clones the catalog on staging, or --branch verify/<model>,
// to the workspace, or updates an existing workspace to the newest of the
// branch it follows; --branch switches it.
func RunWorkspace(args []string) error {
	dir := cli.Value(args, "--root", cli.WorkspaceDir())
	exists := cli.Exists(filepath.Join(dir, ".git"))

	want := BaseBranch
	if exists {
		want = workspaceBranch(dir)
	}
	want = cli.Value(args, "--branch", want)
	if want != BaseBranch && !verifyBranch.MatchString(want) {
		return fmt.Errorf("--branch %q is neither %s nor verify/<vendor>/<model>", want, BaseBranch)
	}

	if !exists {
		if err := cli.Command(
			"git", "clone", "--quiet", "--branch", want,
			cli.Value(args, "--from", upstreamGit), dir,
		); err != nil {
			return err
		}

		fmt.Printf("Workspace %s is on %s.\n", dir, want)
		return nil
	}

	if _, err := cli.Git(dir, "fetch", "--quiet", "origin", want); err != nil {
		return err
	}

	branch, _ := cli.Git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branch != want && !cli.Has(args, "--branch") {
		return fmt.Errorf(
			"workspace %s is not on %s; fetched it without updating",
			dir,
			want,
		)
	}

	upstream := "origin/" + want
	if err := dropMerged(dir, upstream); err != nil {
		return err
	}

	if branch != want {
		// git refuses when local changes would be lost.
		if _, err := cli.Git(dir, "switch", "--quiet", want); err != nil {
			return err
		}
	}

	if _, err := cli.Git(dir, "merge", "--quiet", "--ff-only", upstream); err != nil {
		return err
	}

	head, err := cli.Git(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return err
	}

	fmt.Printf("Workspace %s is on %s at %s.\n", dir, want, head)
	return nil
}

// dropMerged undoes local changes that upstream now holds with the same
// content, such as a contribution that was merged, so updating can
// fast-forward over them.
func dropMerged(dir, upstream string) error {
	out, err := cli.Git(dir, "ls-files", "-z", "--modified", "--others", "--exclude-standard")
	if err != nil {
		return err
	}

	for _, path := range cli.NulSeparated(out) {
		local, err := cli.Git(dir, "hash-object", "--", path)
		if err != nil {
			continue
		}

		merged, err := cli.Git(dir, "rev-parse", "--verify", "--quiet", upstream+":"+path)
		if err != nil || merged != local {
			continue
		}

		if _, err := cli.Git(dir, "cat-file", "-e", "HEAD:"+path); err == nil {
			_, err = cli.Git(dir, "checkout", "--", path)
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

// loadWorkspace loads the catalog of --root, by default the workspace.
func loadWorkspace(args []string) (string, *oddc.Registry, error) {
	root := cli.Value(args, "--root", cli.WorkspaceDir())
	if !cli.Exists(filepath.Join(root, "catalog")) {
		return "", nil, fmt.Errorf("%s holds no catalog; run `oddc workspace` first", root)
	}

	registry, err := oddc.LoadRegistry(root)
	return root, registry, err
}
