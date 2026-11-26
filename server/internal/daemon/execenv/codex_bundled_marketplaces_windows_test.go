//go:build windows

package execenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBundledMarketplacesDirectoryJunctions(t *testing.T) {
	makeJunction := func(t *testing.T, link, target string) {
		t.Helper()
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %s: %v", out, err)
		}
	}

	t.Run("rejects junction in task home path", func(t *testing.T) {
		for _, component := range []string{"codex-home", ".tmp"} {
			t.Run(component, func(t *testing.T) {
				codexHome := filepath.Join(t.TempDir(), "codex-home")
				tmp := filepath.Join(codexHome, ".tmp")
				if component == "codex-home" {
					makeJunction(t, codexHome, t.TempDir())
				} else {
					if err := os.Mkdir(codexHome, 0o755); err != nil {
						t.Fatal(err)
					}
					makeJunction(t, tmp, t.TempDir())
				}
				if err := verifyTaskHomeComponentsForLink(codexHome, tmp); err == nil {
					t.Fatalf("expected %s junction to be refused", component)
				}
			})
		}
	})

	t.Run("retains valid marketplace junction", func(t *testing.T) {
		sharedHome := t.TempDir()
		bundledRoot, _ := writeSharedBundledMarketplace(t, sharedHome)
		codexHome := t.TempDir()
		link := filepath.Join(codexHome, codexBundledMarketplacesRelPath)
		if err := os.Mkdir(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		makeJunction(t, link, bundledRoot)
		if err := exposeSharedCodexBundledMarketplaces(codexHome, sharedHome); err != nil {
			t.Fatal(err)
		}
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeIrregular == 0 || fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("initial junction was replaced: %v, %v", fi, err)
		}
	})
}
