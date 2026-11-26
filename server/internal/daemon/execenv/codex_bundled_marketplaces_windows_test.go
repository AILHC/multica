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

	t.Run("preserves shared marketplace at aliased destination", func(t *testing.T) {
		for _, tc := range []struct{ name, prefix string }{
			{"drive-letter path", ""},
			{"extended path", `\\?\`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				codexHome := t.TempDir()
				bundledRoot, marketplace := writeSharedBundledMarketplace(t, codexHome)
				sharedHome := t.TempDir()
				makeJunction(t, filepath.Join(sharedHome, ".tmp"), filepath.Dir(bundledRoot))

				if err := exposeSharedCodexBundledMarketplaces(tc.prefix+codexHome, sharedHome); err == nil {
					t.Error("expected the shared source aliased to the destination to be refused")
				}
				if _, err := os.Stat(filepath.Join(marketplace, ".agents", "plugins", "marketplace.json")); err != nil {
					t.Fatalf("shared marketplace was lost: %v", err)
				}
			})
		}
	})

	t.Run("unlinks old marketplace when the new home has none", func(t *testing.T) {
		bundledRoot, marketplace := writeSharedBundledMarketplace(t, t.TempDir())
		codexHome := t.TempDir()
		link := filepath.Join(codexHome, codexBundledMarketplacesRelPath)
		if err := os.Mkdir(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		makeJunction(t, link, bundledRoot)

		if err := exposeSharedCodexBundledMarketplaces(codexHome, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Errorf("old shared marketplace is still linked: %v", err)
		}
		if _, err := os.Stat(filepath.Join(marketplace, ".agents", "plugins", "marketplace.json")); err != nil {
			t.Fatalf("old shared marketplace was changed: %v", err)
		}
	})
}
