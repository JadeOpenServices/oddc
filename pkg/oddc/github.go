// SPDX-License-Identifier: GPL-3.0-or-later

package oddc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GitHubSource serves one commit of a GitHub repository. It lists the
// commit's files once and downloads only the files it is asked for.
type GitHubSource struct {
	client *http.Client
	raw    string
	repo   string
	sha    string
	files  []string
}

// NewGitHubSource pins ref, a branch or commit, of repo to its commit.
// api and raw are the GitHub API and raw content base URLs.
func NewGitHubSource(
	client *http.Client,
	api, raw, repo, ref string,
) (*GitHubSource, error) {
	source := &GitHubSource{client: client, raw: raw, repo: repo}

	sha, err := source.get(
		api+"/repos/"+repo+"/commits/"+url.PathEscape(ref),
		"application/vnd.github.sha",
	)
	if err != nil {
		return nil, fmt.Errorf("resolve %s@%s: %w", repo, ref, err)
	}
	source.sha = strings.TrimSpace(string(sha))

	data, err := source.get(
		api+"/repos/"+repo+"/git/trees/"+source.sha+"?recursive=1",
		"application/vnd.github+json",
	)
	if err != nil {
		return nil, fmt.Errorf("list %s@%s: %w", repo, source.sha, err)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("list %s@%s: %w", repo, source.sha, err)
	}
	if tree.Truncated {
		return nil, fmt.Errorf("list %s@%s: file list truncated", repo, source.sha)
	}

	for _, item := range tree.Tree {
		if item.Type == "blob" {
			source.files = append(source.files, item.Path)
		}
	}

	return source, nil
}

func (s *GitHubSource) get(address, accept string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", accept)

	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", address, response.Status)
	}

	return io.ReadAll(response.Body)
}

func (s *GitHubSource) Revision() string {
	return s.sha
}

func (s *GitHubSource) List(dir string) ([]string, error) {
	prefix := strings.TrimSuffix(dir, "/") + "/"

	var files []string
	for _, file := range s.files {
		if strings.HasPrefix(file, prefix) {
			files = append(files, file)
		}
	}

	return files, nil
}

func (s *GitHubSource) Read(file string) ([]byte, error) {
	return s.get(s.raw+"/"+s.repo+"/"+s.sha+"/"+file, "*/*")
}
