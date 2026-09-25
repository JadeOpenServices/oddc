package oddc

import (
	"errors"
	"testing"
)

func TestRepositoryFrameworkModelMatch(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	id, err := registry.MatchModel(MachineIdentity{
		FormFactor:  "laptop",
		SysVendor:   "Framework",
		ProductName: "Laptop 13 (AMD Ryzen 7040Series)",
	})
	if err != nil {
		t.Fatal(err)
	}

	if id != "model/framework/laptop-13-amd-ryzen-7040" {
		t.Fatalf("matched %q", id)
	}
}

func TestRepositoryHPModelMatch(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	id, err := registry.MatchModel(MachineIdentity{
		FormFactor:  "laptop",
		SysVendor:   "HP",
		ProductName: "HP ZBook x2 G4",
		BoardName:   "824C",
	})
	if err != nil {
		t.Fatal(err)
	}

	if id != "model/hp/zbook-x2-g4" {
		t.Fatalf("matched %q", id)
	}
}

func TestRepositoryUnknownModelDoesNotGuess(t *testing.T) {
	registry, err := LoadRegistry(repositoryODDCRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.MatchModel(MachineIdentity{
		FormFactor:  "laptop",
		SysVendor:   "Unknown",
		ProductName: "Unknown Laptop",
	})
	if !errors.Is(err, ErrNoModelMatch) {
		t.Fatalf("err=%v", err)
	}
}
