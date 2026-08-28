package execenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSymlink_SkipsWhenSourceMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "missing.json")
	dst := filepath.Join(dir, "link.json")

	if err := ensureSymlink(src, dst); err != nil {
		t.Fatalf("ensureSymlink: %v", err)
	}

	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Error("expected dst to not be created when src is missing")
	}
}

func TestEnsureSymlink_RemovesStaleDestinationWhenSourceMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "missing.json")
	dst := filepath.Join(dir, "stale.json")
	if err := os.WriteFile(dst, []byte("stale credentials"), 0o600); err != nil {
		t.Fatalf("seed stale destination: %v", err)
	}

	if err := ensureSymlink(src, dst); err != nil {
		t.Fatalf("ensureSymlink: %v", err)
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Fatalf("stale destination still exists after source removal: %v", err)
	}
}

func TestPrepareCodexHome_RemovesAuthFromPreviouslySelectedSharedHome(t *testing.T) {
	t.Parallel()
	oldSharedHome := t.TempDir()
	newSharedHome := t.TempDir()
	taskHome := filepath.Join(t.TempDir(), "codex-home")

	if err := os.WriteFile(filepath.Join(oldSharedHome, "auth.json"), []byte("old profile credentials"), 0o600); err != nil {
		t.Fatalf("seed old shared auth: %v", err)
	}
	if err := prepareCodexHomeWithOpts(taskHome, CodexHomeOptions{SharedCodexHome: oldSharedHome}, testLogger()); err != nil {
		t.Fatalf("prepare with old shared home: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(taskHome, "auth.json")); err != nil {
		t.Fatalf("task auth missing after initial prepare: %v", err)
	}

	if err := prepareCodexHomeWithOpts(taskHome, CodexHomeOptions{SharedCodexHome: newSharedHome}, testLogger()); err != nil {
		t.Fatalf("reuse with new shared home: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(taskHome, "auth.json")); !os.IsNotExist(err) {
		t.Fatalf("old shared auth remains after profile switch: %v", err)
	}
}

func TestPrepareCodexHome_UsesExplicitSharedHomeForCaches(t *testing.T) {
	explicitHome := t.TempDir()
	environmentHome := t.TempDir()
	taskHome := filepath.Join(t.TempDir(), "codex-home")
	t.Setenv("CODEX_HOME", environmentHome)

	if err := os.WriteFile(filepath.Join(explicitHome, codexModelsCacheFile), []byte(`{"source":"explicit"}`), 0o600); err != nil {
		t.Fatalf("write explicit models cache: %v", err)
	}
	explicitPlugin := filepath.Join(explicitHome, "plugins", "cache", "explicit-plugin")
	if err := os.MkdirAll(explicitPlugin, 0o755); err != nil {
		t.Fatalf("create explicit plugin cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(explicitPlugin, "marker.txt"), []byte("explicit"), 0o600); err != nil {
		t.Fatalf("write explicit plugin marker: %v", err)
	}

	if err := os.WriteFile(filepath.Join(environmentHome, codexModelsCacheFile), []byte(`{"source":"environment"}`), 0o600); err != nil {
		t.Fatalf("write environment models cache: %v", err)
	}
	environmentPlugin := filepath.Join(environmentHome, "plugins", "cache", "environment-plugin")
	if err := os.MkdirAll(environmentPlugin, 0o755); err != nil {
		t.Fatalf("create environment plugin cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(environmentPlugin, "marker.txt"), []byte("environment"), 0o600); err != nil {
		t.Fatalf("write environment plugin marker: %v", err)
	}

	if err := prepareCodexHomeWithOpts(taskHome, CodexHomeOptions{SharedCodexHome: explicitHome}, testLogger()); err != nil {
		t.Fatalf("prepare with explicit shared home: %v", err)
	}
	modelsCache, err := os.ReadFile(filepath.Join(taskHome, codexModelsCacheFile))
	if err != nil {
		t.Fatalf("read task models cache: %v", err)
	}
	if string(modelsCache) != `{"source":"explicit"}` {
		t.Fatalf("task models cache = %q, want explicit shared cache", modelsCache)
	}
	pluginMarker, err := os.ReadFile(filepath.Join(taskHome, "plugins", "cache", "explicit-plugin", "marker.txt"))
	if err != nil {
		t.Fatalf("read explicit plugin marker through task cache: %v", err)
	}
	if string(pluginMarker) != "explicit" {
		t.Fatalf("plugin marker = %q, want explicit shared cache", pluginMarker)
	}
	if _, err := os.Lstat(filepath.Join(taskHome, "plugins", "cache", "environment-plugin")); !os.IsNotExist(err) {
		t.Fatalf("environment plugin cache leaked into task home: %v", err)
	}
}

func TestEnsureSymlink_ReplacesStaleRegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "existing.json")
	os.WriteFile(src, []byte("new"), 0o644)
	os.WriteFile(dst, []byte("old"), 0o644)

	// Regression for issue #2081: a regular file at dst (e.g. left over from
	// the Windows copy fallback in createFileLink) must be replaced so the
	// per-task home picks up changes to the shared source — otherwise a
	// once-stale auth.json never refreshes across env reuses.
	if err := ensureSymlink(src, dst); err != nil {
		t.Fatalf("ensureSymlink: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "new" {
		t.Errorf("dst content = %q, want %q (file should be re-linked/re-copied from src)", data, "new")
	}
}

func TestEnsureSymlink_RefreshesAfterCopyFallbackThenSrcChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "auth.json")
	dst := filepath.Join(dir, "task-auth.json")

	// Simulate the Windows copy fallback: first link is a copy of v1.
	os.WriteFile(src, []byte(`{"refresh_token":"v1"}`), 0o644)
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("seed copy fallback: %v", err)
	}

	// Shared source rotates to v2 (e.g. Codex Desktop refreshed the token).
	os.WriteFile(src, []byte(`{"refresh_token":"v2"}`), 0o644)

	// Reuse path runs ensureSymlink again — expected to refresh dst from src.
	if err := ensureSymlink(src, dst); err != nil {
		t.Fatalf("ensureSymlink: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != `{"refresh_token":"v2"}` {
		t.Errorf("dst content after refresh = %q, want v2 contents", data)
	}
}

func TestCreateDirLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("hello"), 0o644)

	if err := createDirLink(src, dst); err != nil {
		t.Fatalf("createDirLink: %v", err)
	}

	// Should be able to read files through the link.
	data, err := os.ReadFile(filepath.Join(dst, "test.txt"))
	if err != nil {
		t.Fatalf("read through link: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", data, "hello")
	}
}

func TestCreateFileLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "link.json")
	os.WriteFile(src, []byte(`{"key":"value"}`), 0o644)

	if err := createFileLink(src, dst); err != nil {
		t.Fatalf("createFileLink: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read link: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("content = %q", data)
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	os.WriteFile(src, []byte("content"), 0o644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "content" {
		t.Errorf("content = %q", data)
	}

	// Verify it's a copy, not a symlink.
	fi, _ := os.Lstat(dst)
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("expected regular file, not symlink")
	}
}
