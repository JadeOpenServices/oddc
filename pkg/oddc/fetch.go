package oddc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
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

// fetcher reads entities from a source at most once.
type fetcher struct {
	source   Source
	entities map[string][]byte
}

func newFetcher(source Source) *fetcher {
	return &fetcher{source: source, entities: map[string][]byte{}}
}

// entity reads an entity by ID from its address.
func (f *fetcher) entity(id string) (Entity, error) {
	file := "catalog/entities/" + id + ".json"

	data, cached := f.entities[id]
	if !cached {
		var err error
		if data, err = f.source.Read(file); err != nil {
			return Entity{}, fmt.Errorf("fetch %s: %w", id, err)
		}
	}

	entity, err := decodeEntityData(file, data)
	if err != nil {
		return Entity{}, err
	}

	f.entities[id] = data
	return entity, nil
}

// closure returns id and every entity it references, transitively.
func (f *fetcher) closure(id string) ([]string, error) {
	seen := map[string]bool{id: true}
	queue := []string{id}

	for len(queue) > 0 {
		entity, err := f.entity(queue[0])
		if err != nil {
			return nil, err
		}
		queue = queue[1:]

		for _, ref := range collectRefs(entity.Data) {
			if !seen[ref] {
				seen[ref] = true
				queue = append(queue, ref)
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids, nil
}

// write puts the entities, in canonical layout, below root.
func (f *fetcher) write(root string, ids []string) error {
	for _, id := range ids {
		file := EntityPath(filepath.Join(root, "catalog", "entities"), id)

		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, f.entities[id], 0o644); err != nil {
			return err
		}
	}

	return nil
}

// match finds the machine's model. A model owns its DMI identity, so its
// own data rules it out before any of its references are fetched; only the
// remaining candidates are fetched and matched in full.
func (f *fetcher) match(identity MachineIdentity) (string, error) {
	files, err := f.source.List("catalog/entities/model")
	if err != nil {
		return "", err
	}

	unknownFormFactor := identity
	unknownFormFactor.FormFactor = ""

	var candidates []string

	for _, file := range files {
		if path.Ext(file) != ".json" {
			continue
		}

		id := strings.TrimSuffix(strings.TrimPrefix(file, "catalog/entities/"), ".json")

		entity, err := f.entity(id)
		if err != nil {
			return "", err
		}

		_, inherited := entity.Data["ref"]
		inherited = inherited || len(collectRefs(entity.Data["identity"])) > 0

		if _, matched := identityScore(entity.Data, unknownFormFactor); matched || inherited {
			candidates = append(candidates, id)
		}
	}

	if len(candidates) == 0 {
		return "", ErrNoModelMatch
	}

	root, err := os.MkdirTemp("", "oddc-match-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(root)

	for _, candidate := range candidates {
		ids, err := f.closure(candidate)
		if err != nil {
			return "", err
		}

		if err := f.write(root, ids); err != nil {
			return "", err
		}
	}

	registry, err := LoadRegistry(root)
	if err != nil {
		return "", err
	}

	return registry.MatchModel(identity)
}

// answer writes model's reference closure, its evidence and the source
// revision to out, which must not exist yet. A failed fetch leaves no out.
func (f *fetcher) answer(model, out string) error {
	if _, err := os.Lstat(out); err == nil {
		return fmt.Errorf("%s already exists", out)
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}

	staging, err := os.MkdirTemp(filepath.Dir(out), ".oddc-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	ids, err := f.closure(model)
	if err != nil {
		return err
	}

	if err := f.write(staging, ids); err != nil {
		return err
	}

	evidence, err := f.source.List("evidence/" + model)
	if err != nil {
		return err
	}

	for _, file := range evidence {
		data, err := f.source.Read(file)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", file, err)
		}

		target := filepath.Join(staging, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}

	if err := os.WriteFile(
		filepath.Join(staging, "revision"),
		[]byte(f.source.Revision()+"\n"),
		0o644,
	); err != nil {
		return err
	}

	registry, err := LoadRegistry(staging)
	if err != nil {
		return err
	}
	if entity := registry.Entities[model]; entity.Kind != "DeviceModel" {
		return fmt.Errorf("%q is a %s, not a DeviceModel", model, entity.Kind)
	}

	if err := os.Chmod(staging, 0o755); err != nil {
		return err
	}

	return os.Rename(staging, out)
}

// Match returns the model a machine with this identity is, reading only
// the source's model files and the references of plausible models.
func Match(source Source, identity MachineIdentity) (string, error) {
	return newFetcher(source).match(identity)
}

// Fetch matches the machine and writes only its model to out: the model's
// reference closure in canonical layout, its evidence and the revision.
func Fetch(source Source, identity MachineIdentity, out string) (string, error) {
	f := newFetcher(source)

	model, err := f.match(identity)
	if err != nil {
		return "", err
	}

	return model, f.answer(model, out)
}

// FetchModel writes one model, chosen by ID, to out as Fetch does.
func FetchModel(source Source, model, out string) error {
	return newFetcher(source).answer(model, out)
}
