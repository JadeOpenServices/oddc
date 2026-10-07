package contribute

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/JadeOpenServices/oddc/internal/catalog"
	"github.com/JadeOpenServices/oddc/internal/cli"
)

// contribution lists the workspace files that differ from base. It
// refuses files outside catalog/ and evidence/.
func contribution(root, base string) ([]string, error) {
	changed, err := cli.Git(root, "diff", "-z", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}

	untracked, err := cli.Git(root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	paths := append(cli.NulSeparated(changed), cli.NulSeparated(untracked)...)
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
		if _, err := cli.Git(root, "cat-file", "-e", base+":"+path); err == nil {
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

// Prepared is a checked contribution: the tree it sends, built on the
// newest staging.
type Prepared struct {
	Base, Tree, Branch, Title, Body string
	Paths                           []string
}

// Prepare checks the workspace's changes and builds their
// tree on the newest staging in a separate index, so the workspace itself
// stays as it is. Nothing leaves the machine unless the catalog validates
// and evidence was only added.
func Prepare(args []string) (Prepared, error) {
	root := cli.Value(args, "--root", cli.WorkspaceDir())
	p := Prepared{Base: "origin/" + BaseBranch}

	if _, err := cli.Git(root, "fetch", "--quiet", "origin", BaseBranch); err != nil {
		return p, err
	}

	paths, err := contribution(root, p.Base)
	if err != nil {
		return p, err
	}
	if len(paths) == 0 {
		return p, fmt.Errorf("nothing to contribute: catalog/ and evidence/ match %s", BaseBranch)
	}
	p.Paths = paths

	if err := catalog.RunValidate(root, p.Base, false); err != nil {
		return p, err
	}

	p.Title = cli.Value(args, "--title", pullRequestTitle(root, p.Base, paths))
	p.Body = cli.Value(args, "--body", "Files:\n\n- "+strings.Join(paths, "\n- ")+"\n\nSent with `oddc contribute`.")

	index, err := os.MkdirTemp("", "oddc-contribute-")
	if err != nil {
		return p, err
	}
	defer os.RemoveAll(index)
	indexEnv := []string{"GIT_INDEX_FILE=" + filepath.Join(index, "index")}

	if _, err := cli.GitEnv(root, indexEnv, "read-tree", p.Base); err != nil {
		return p, err
	}
	if _, err := cli.GitEnv(root, indexEnv, append([]string{"add", "--all", "--"}, paths...)...); err != nil {
		return p, err
	}
	if p.Tree, err = cli.GitEnv(root, indexEnv, "write-tree"); err != nil {
		return p, err
	}

	// The branch is named after the contributed tree, so a contribution
	// that was already sent is not pushed twice.
	p.Branch = "contrib/" + p.Tree[:12]

	return p, nil
}
