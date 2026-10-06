package oddc

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSys(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadIdentityMatchesHP(t *testing.T) {
	sys := t.TempDir()
	writeSys(t, sys, map[string]string{
		"class/dmi/id/sys_vendor":   "HP",
		"class/dmi/id/product_name": "HP ZBook x2 G4",
		"class/dmi/id/board_name":   "824C",
		"class/dmi/id/chassis_type": "32",
	})

	identity := ReadIdentity(sys)
	if identity.FormFactor != "laptop" || identity.BoardName != "824C" {
		t.Fatalf("identity = %+v", identity)
	}

	registry, err := LoadRegistry(".")
	if err != nil {
		t.Fatal(err)
	}

	model, err := registry.MatchModel(identity)
	if err != nil || model != "model/hp/zbook-x2-g4" {
		t.Fatalf("model = %q, err = %v", model, err)
	}
}

func TestReadIdentityDesktopChassis(t *testing.T) {
	sys := t.TempDir()
	writeSys(t, sys, map[string]string{
		"class/dmi/id/chassis_type": "3",
	})

	if got := ReadIdentity(sys).FormFactor; got != "desktop" {
		t.Fatalf("form factor = %q", got)
	}
}
