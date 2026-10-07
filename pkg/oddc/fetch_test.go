package oddc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// closureOf lists a model and every entity it references, transitively.
func closureOf(registry *Registry, id string) []string {
	seen := map[string]bool{}
	queue := []string{id}

	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if seen[next] {
			continue
		}
		seen[next] = true
		queue = append(queue, collectRefs(registry.Entities[next].Data)...)
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids
}

// filesBelow returns the slash-separated files below root, relative to it.
func filesBelow(t *testing.T, root string) []string {
	t.Helper()

	files, err := DirSource{Root: root}.List(".")
	if err != nil {
		t.Fatal(err)
	}
	for i, file := range files {
		files[i] = strings.TrimPrefix(file, "./")
	}
	sort.Strings(files)

	return files
}

// expectAnswer checks that out holds exactly model's closure and evidence,
// byte for byte as in the repository, and the revision.
func expectAnswer(t *testing.T, registry *Registry, model, out, revision string) {
	t.Helper()

	want := []string{"revision"}
	for _, id := range closureOf(registry, model) {
		want = append(want, "catalog/entities/"+id+".json")
	}
	evidence, err := DirSource{Root: "."}.List("evidence/" + model)
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, evidence...)
	sort.Strings(want)

	got := filesBelow(t, out)
	if !slices.Equal(got, want) {
		t.Fatalf("%s answer holds\n%v\nwant\n%v", model, got, want)
	}

	for _, file := range got {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}

		if file == "revision" {
			if string(data) != revision+"\n" {
				t.Errorf("%s revision %q, want %q", model, data, revision)
			}
			continue
		}

		source, err := os.ReadFile(filepath.FromSlash(file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, source) {
			t.Errorf("%s: %s differs from the repository", model, file)
		}
	}

	answer, err := LoadRegistry(out)
	if err != nil {
		t.Fatalf("%s answer invalid: %v", model, err)
	}
	if matched, err := answer.MatchModel(mustIdentity(t, registry, model)); err != nil || matched != model {
		t.Errorf("%s answer matches %q, %v", model, matched, err)
	}
}

func mustIdentity(t *testing.T, registry *Registry, model string) MachineIdentity {
	t.Helper()

	identity, err := registry.ModelIdentity(model)
	if err != nil {
		t.Fatal(err)
	}

	return identity
}

func TestFetchEveryModelFromCheckout(t *testing.T) {
	registry := catalog(t)

	for _, model := range models(t, registry) {
		out := filepath.Join(t.TempDir(), "oddc")

		matched, err := Fetch(DirSource{Root: "."}, mustIdentity(t, registry, model), out)
		if err != nil || matched != model {
			t.Fatalf("Fetch for %s = %q, %v", model, matched, err)
		}

		expectAnswer(t, registry, model, out, "local")
	}
}

func TestFetchModelByID(t *testing.T) {
	registry := catalog(t)

	for _, model := range models(t, registry) {
		out := filepath.Join(t.TempDir(), "oddc")

		if err := FetchModel(DirSource{Root: "."}, model, out); err != nil {
			t.Fatal(err)
		}

		expectAnswer(t, registry, model, out, "local")
	}
}

func TestFetchModelRefusesNonModels(t *testing.T) {
	registry := catalog(t)

	for id, entity := range registry.Entities {
		if entity.Kind == "DeviceModel" {
			continue
		}

		out := filepath.Join(t.TempDir(), "oddc")
		if err := FetchModel(DirSource{Root: "."}, id, out); err == nil {
			t.Errorf("fetched %s, a %s, as a model", id, entity.Kind)
		}
		if _, err := os.Lstat(out); err == nil {
			t.Errorf("failed fetch of %s left %s", id, out)
		}
	}
}

func TestFetchUnknownMachineWritesNothing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "oddc")

	if _, err := Fetch(DirSource{Root: "."}, MachineIdentity{}, out); !errors.Is(err, ErrNoModelMatch) {
		t.Fatalf("empty identity: %v, want ErrNoModelMatch", err)
	}
	if _, err := os.Lstat(out); err == nil {
		t.Errorf("unmatched fetch left %s", out)
	}
}

func TestFetchKeepsExistingAnswer(t *testing.T) {
	registry := catalog(t)
	model := models(t, registry)[0]
	out := filepath.Join(t.TempDir(), "oddc")

	if err := FetchModel(DirSource{Root: "."}, model, out); err != nil {
		t.Fatal(err)
	}
	if err := FetchModel(DirSource{Root: "."}, model, out); err == nil {
		t.Error("fetch overwrote an existing answer")
	}
}

// github serves the repository checkout as GitHub serves a commit of it
// and records which files clients download.
type github struct {
	sha        string
	files      []string
	mu         sync.Mutex
	downloaded []string
}

func newGitHub(t *testing.T) (*github, *httptest.Server) {
	t.Helper()

	hub := &github{}
	hash := sha256.New()
	for _, file := range filesBelow(t, ".") {
		if strings.HasPrefix(file, ".git/") {
			continue
		}

		data, err := os.ReadFile(filepath.FromSlash(file))
		if err != nil {
			t.Fatal(err)
		}

		hub.files = append(hub.files, file)
		hash.Write([]byte(file))
		hash.Write(data)
	}
	// A content address of the served tree stands in for the commit ID.
	hub.sha = hex.EncodeToString(hash.Sum(nil))[:40]

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/"+Repository+"/commits/staging", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.sha" {
			http.Error(w, "want sha media type", http.StatusUnsupportedMediaType)
			return
		}
		w.Write([]byte(hub.sha))
	})
	mux.HandleFunc("GET /repos/"+Repository+"/git/trees/"+hub.sha, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "1" {
			http.Error(w, "want recursive", http.StatusBadRequest)
			return
		}

		type item struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		var tree []item
		for _, file := range hub.files {
			tree = append(tree, item{file, "blob"})
		}
		json.NewEncoder(w).Encode(map[string]any{"sha": hub.sha, "tree": tree, "truncated": false})
	})
	mux.HandleFunc("GET /"+Repository+"/"+hub.sha+"/", func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/"+Repository+"/"+hub.sha+"/")
		if !slices.Contains(hub.files, file) {
			http.NotFound(w, r)
			return
		}

		hub.mu.Lock()
		hub.downloaded = append(hub.downloaded, file)
		hub.mu.Unlock()

		http.ServeFile(w, r, filepath.FromSlash(file))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return hub, server
}

func TestFetchEveryModelFromGitHub(t *testing.T) {
	registry := catalog(t)

	var catalogFiles int
	for _, root := range []string{"catalog", "evidence"} {
		files, err := DirSource{Root: "."}.List(root)
		if err != nil {
			t.Fatal(err)
		}
		catalogFiles += len(files)
	}

	for _, model := range models(t, registry) {
		hub, server := newGitHub(t)

		source, err := NewGitHubSource(server.Client(), server.URL, server.URL, Repository, "staging")
		if err != nil {
			t.Fatal(err)
		}

		out := filepath.Join(t.TempDir(), "oddc")
		matched, err := Fetch(source, mustIdentity(t, registry, model), out)
		if err != nil || matched != model {
			t.Fatalf("Fetch for %s = %q, %v", model, matched, err)
		}

		expectAnswer(t, registry, model, out, hub.sha)

		// Besides model files, only the closures of models whose own
		// identity does not rule them out may be downloaded.
		identity := mustIdentity(t, registry, model)
		identity.FormFactor = ""
		allowed := map[string]bool{}
		for _, other := range models(t, registry) {
			if _, plausible := identityScore(registry.Entities[other].Data, identity); plausible || other == model {
				for _, id := range closureOf(registry, other) {
					allowed["catalog/entities/"+id+".json"] = true
				}
			}
		}

		for _, file := range hub.downloaded {
			if strings.HasPrefix(file, "evidence/") &&
				!strings.HasPrefix(file, "evidence/"+model+"/") {
				t.Errorf("%s fetch downloaded another model's evidence %s", model, file)
			}
			if strings.HasPrefix(file, "catalog/entities/") &&
				!strings.HasPrefix(file, "catalog/entities/model/") &&
				!allowed[file] {
				t.Errorf("%s fetch downloaded unrelated %s", model, file)
			}
			if !strings.HasPrefix(file, "catalog/entities/") &&
				!strings.HasPrefix(file, "evidence/") {
				t.Errorf("%s fetch downloaded %s", model, file)
			}
		}
		if len(hub.downloaded) >= catalogFiles {
			t.Errorf("%s fetch downloaded %d of %d catalog files", model, len(hub.downloaded), catalogFiles)
		}
	}
}

func TestGitHubSourceRejectsUnknownRef(t *testing.T) {
	_, server := newGitHub(t)

	if _, err := NewGitHubSource(server.Client(), server.URL, server.URL, Repository, "main"); err == nil {
		t.Error("resolved a ref the server does not have")
	}
}

func TestDirSourceListsNothingForMissingDir(t *testing.T) {
	files, err := DirSource{Root: "."}.List("evidence/does-not-exist")
	if err != nil || len(files) != 0 {
		t.Errorf("List = %v, %v; want nothing", files, err)
	}
}
