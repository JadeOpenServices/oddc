// SPDX-License-Identifier: GPL-3.0-or-later

package contribute

import (
	"fmt"
	"path"
	"time"

	"github.com/JadeOpenServices/oddc/internal/cli"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
)

// RunContribute sends the workspace's checked changes as a pull request
// against the branch the workspace follows, staging or verify/<model>,
// from the contributor's GitHub account, through a fork when the account
// cannot push to the catalog. The commit's author is the
// account's noreply address and its dates are in UTC.
func RunContribute(args []string) error {
	root := cli.Value(args, "--root", cli.WorkspaceDir())

	p, err := Prepare(args)
	if err != nil {
		return err
	}

	user, err := cli.Gh("api", "user", "--jq", `.login + " " + (.id | tostring)`)
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
	commit, err := cli.GitEnv(root, []string{
		"GIT_AUTHOR_NAME=" + login, "GIT_AUTHOR_EMAIL=" + email, "GIT_AUTHOR_DATE=" + now,
		"GIT_COMMITTER_NAME=" + login, "GIT_COMMITTER_EMAIL=" + email, "GIT_COMMITTER_DATE=" + now,
	}, "commit-tree", p.Tree, "-p", p.Base, "-m", p.Title+"\n\n"+p.Body)
	if err != nil {
		return err
	}

	repo, head := oddc.Repository, p.Branch
	push, err := cli.Gh("api", "repos/"+repo, "--jq", ".permissions.push")
	if err != nil {
		return err
	}
	if push != "true" {
		if _, err := cli.Gh("repo", "fork", repo, "--clone=false", "--remote=false"); err != nil {
			return err
		}
		repo, head = login+"/"+path.Base(oddc.Repository), login+":"+p.Branch
	}

	remote := "https://github.com/" + repo + ".git"
	sent, err := cli.Git(root, "ls-remote", remote, "refs/heads/"+p.Branch)
	if err != nil {
		return err
	}
	if sent == "" {
		if _, err := cli.Git(root, "push", "--quiet", remote, commit+":refs/heads/"+p.Branch); err != nil {
			return err
		}
	}

	url, err := cli.Gh(
		"pr", "create", "--repo", oddc.Repository, "--base", p.Target, "--head", head,
		"--title", p.Title, "--body", p.Body,
	)
	if err != nil {
		return err
	}

	fmt.Println(url)
	return nil
}
