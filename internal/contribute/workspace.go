// Package contribute holds the contributor commands. A contribution is
// data: files below catalog/ and evidence/ of a workspace checkout, sent as
// one pull request against staging from the contributor's own GitHub
// account.
package contribute

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

const (
	upstreamGit = "https://github.com/" + oddc.Repository + ".git"
	BaseBranch  = "staging"
)

// RunWorkspace clones the catalog on staging to the workspace, or updates
// an existing workspace to the newest staging.
func RunWorkspace(args []string) error {
	dir := cli.Value(args, "--root", cli.WorkspaceDir())

	if !cli.Exists(filepath.Join(dir, ".git")) {
		if err := cli.Command(
			"git", "clone", "--quiet", "--branch", BaseBranch,
			cli.Value(args, "--from", upstreamGit), dir,
		); err != nil {
			return err
		}

		fmt.Printf("Workspace %s is on %s.\n", dir, BaseBranch)
		return nil
	}

	if _, err := cli.Git(dir, "fetch", "--quiet", "origin", BaseBranch); err != nil {
		return err
	}

	branch, _ := cli.Git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branch != BaseBranch {
		return fmt.Errorf(
			"workspace %s is not on %s; fetched it without updating",
			dir,
			BaseBranch,
		)
	}

	upstream := "origin/" + BaseBranch
	if err := dropMerged(dir, upstream); err != nil {
		return err
	}

	if _, err := cli.Git(dir, "merge", "--quiet", "--ff-only", upstream); err != nil {
		return err
	}

	head, err := cli.Git(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return err
	}

	fmt.Printf("Workspace %s is on %s at %s.\n", dir, BaseBranch, head)
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
