// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitSource serves one commit of a git repository. It fetches the
// commit's trees without file contents, then each file only when it is
// read. git checks every object it fetches against the commit.
type GitSource struct {
	dir   string
	sha   string
	files []string
}

// NewGitSource pins ref, a branch or full commit ID, of remote to its
// commit. It keeps what it fetches in dir, a bare repository it creates.
func NewGitSource(dir, remote, ref string) (*GitSource, error) {
	source := &GitSource{dir: dir}

	if _, err := gitRun("", "init", "-q", "--bare", dir); err != nil {
		return nil, err
	}
	for _, args := range [][]string{
		{"remote", "add", "origin", remote},
		{"fetch", "-q", "--no-tags", "--depth=1", "--filter=blob:none", "origin", ref},
	} {
		if _, err := gitRun(dir, args...); err != nil {
			return nil, fmt.Errorf("resolve %s@%s: %w", remote, ref, err)
		}
	}

	sha, err := gitRun(dir, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("resolve %s@%s: %w", remote, ref, err)
	}
	source.sha = strings.TrimSpace(string(sha))

	list, err := gitRun(dir, "ls-tree", "-r", "-z", "--name-only", source.sha)
	if err != nil {
		return nil, fmt.Errorf("list %s@%s: %w", remote, source.sha, err)
	}
	for _, file := range strings.Split(string(list), "\x00") {
		if file != "" {
			source.files = append(source.files, file)
		}
	}

	return source, nil
}

// gitRun runs git in dir, or the current directory when dir is empty,
// without asking for credentials.
func gitRun(dir string, args ...string) ([]byte, error) {
	command := args[0]
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", command, err, strings.TrimSpace(stderr.String()))
	}

	return out, nil
}

func (s *GitSource) Revision() string {
	return s.sha
}

func (s *GitSource) List(dir string) ([]string, error) {
	prefix := strings.TrimSuffix(dir, "/") + "/"

	var files []string
	for _, file := range s.files {
		if strings.HasPrefix(file, prefix) {
			files = append(files, file)
		}
	}

	return files, nil
}

func (s *GitSource) Read(file string) ([]byte, error) {
	return gitRun(s.dir, "cat-file", "blob", s.sha+":"+file)
}
