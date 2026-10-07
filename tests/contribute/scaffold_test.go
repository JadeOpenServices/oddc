package contribute_test

import (
	"testing"

	"github.com/JadeOpenServices/oddc/internal/contribute"
	"github.com/JadeOpenServices/oddc/pkg/oddc"
	"github.com/JadeOpenServices/oddc/tests/fixture"
)

// Scaffolding the Framework 13 into a catalog that lacks it redrafts it
// from its facts: same identity, and its components where they were.
func TestScaffoldRedraftsCatalogModel(t *testing.T) {
	_, workspace := catalogUpstream(t, framework13)
	full, _ := fixture.Catalog(t)
	facts := fixture.FactsFile(t, full, framework13)
	args := []string{"scaffold", "--root", workspace, "--facts", facts, "--id", framework13}

	if err := contribute.RunScaffold(args); err != nil {
		t.Fatal(err)
	}

	registry, err := oddc.LoadRegistry(workspace)
	if err != nil {
		t.Fatal(err)
	}

	modelFacts, err := full.ModelFacts(framework13)
	if err != nil {
		t.Fatal(err)
	}
	classification, err := registry.Classify(modelFacts)
	if err != nil {
		t.Fatal(err)
	}
	if classification.Model != framework13 {
		t.Fatalf("draft does not classify: %+v", classification)
	}

	drafted, original := map[string]string{}, map[string]string{}
	refs(registry.Entities[framework13].Data, "", drafted)
	refs(full.Entities[framework13].Data, "", original)
	for _, path := range []string{"vendor", "class"} {
		if drafted[path] != original[path] {
			t.Errorf("%s = %q want %q", path, drafted[path], original[path])
		}
	}
	for path, ref := range drafted {
		if original[path] != ref {
			t.Errorf("drafted %s at %s, which the catalog model has as %q", ref, path, original[path])
		}
	}

	if err := contribute.RunScaffold(args); err == nil {
		t.Fatal("scaffolded a machine that already matches")
	}
}
