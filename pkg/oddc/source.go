// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Upstream is where ODDC publishes its catalog.
const (
	Repository = "JadeOpenServices/oddc"
	Remote     = "https://github.com/" + Repository + ".git"
)

// Source serves catalog files by their slash-separated repository path,
// all from one revision.
type Source interface {
	Revision() string
	// List returns the paths of all files below dir; none when dir is absent.
	List(dir string) ([]string, error)
	Read(file string) ([]byte, error)
}

// DirSource serves a catalog checkout or a fetched answer.
type DirSource struct {
	Root string
}

// Revision is the recorded revision of a fetched answer, else the commit
// checked out at Root when its catalog, schemas and evidence are exactly
// that commit's, else "local": never a commit the files differ from.
func (s DirSource) Revision() string {
	data, err := os.ReadFile(filepath.Join(s.Root, "revision"))
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	git := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", s.Root}, args...)...).Output()
		return strings.TrimSpace(string(out)), err
	}

	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return "local"
	}
	changed, err := git("status", "--porcelain", "--untracked-files=all", "--", "catalog", "schemas", "evidence")
	if err != nil || changed != "" {
		return "local"
	}

	return head
}

func (s DirSource) List(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(
		filepath.Join(s.Root, filepath.FromSlash(dir)),
		func(file string, entry fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			if err != nil || entry.IsDir() {
				return err
			}

			relative, err := filepath.Rel(s.Root, file)
			if err != nil {
				return err
			}

			files = append(files, filepath.ToSlash(relative))
			return nil
		},
	)

	return files, err
}

func (s DirSource) Read(file string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(file)))
}
