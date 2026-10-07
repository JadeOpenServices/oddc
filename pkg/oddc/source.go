// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Upstream is where ODDC publishes its catalog.
const (
	GitHubAPI  = "https://api.github.com"
	GitHubRaw  = "https://raw.githubusercontent.com"
	Repository = "JadeOpenServices/oddc"
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

// Revision is the recorded revision of a fetched answer, else "local".
func (s DirSource) Revision() string {
	data, err := os.ReadFile(filepath.Join(s.Root, "revision"))
	if err != nil {
		return "local"
	}

	return strings.TrimSpace(string(data))
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
