package execenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexSessionWriterLockUsesExplicitSharedHome(t *testing.T) {
	sharedHome := t.TempDir()
	ambientHome := t.TempDir()
	t.Setenv("CODEX_HOME", ambientHome)
	store := filepath.Join(sharedHome, "sessions")
	first, err := AcquireCodexSessionWriterLockAt(sharedHome, context.Background(), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := os.Stat(codexSessionWriterLockPath(sharedHome, store)); err != nil {
		t.Fatalf("explicit shared home lock missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ambientHome, codexSessionWriterLockRoot)); !os.IsNotExist(err) {
		t.Fatalf("ambient home was used for locking: %v", err)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	second, err := AcquireCodexSessionWriterLockAt(sharedHome, ctx, store, nil)
	if second != nil {
		second.Release()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("same explicit store must remain locked after ambient home changes: %v", err)
	}
}

func TestCodexSessionWriterLockSerializesOneStore(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	store := filepath.Join(t.TempDir(), "sessions")

	first, err := AcquireCodexSessionWriterLock(context.Background(), store, nil)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer first.Release()

	waiting := make(chan struct{})
	acquired := make(chan *CodexSessionWriterLock, 1)
	errs := make(chan error, 1)
	go func() {
		lock, err := AcquireCodexSessionWriterLock(context.Background(), store, func() { close(waiting) })
		if err != nil {
			errs <- err
			return
		}
		acquired <- lock
	}()

	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("second writer never reported waiting")
	}
	select {
	case lock := <-acquired:
		lock.Release()
		t.Fatal("second writer acquired while the first still held the store")
	case err := <-errs:
		t.Fatalf("second writer failed while waiting: %v", err)
	case <-time.After(75 * time.Millisecond):
	}

	first.Release()
	select {
	case lock := <-acquired:
		lock.Release()
	case err := <-errs:
		t.Fatalf("second writer failed after release: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("second writer did not acquire after release")
	}
}

func TestCodexSessionWriterLockCancellationStopsWait(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	store := filepath.Join(t.TempDir(), "sessions")

	first, err := AcquireCodexSessionWriterLock(context.Background(), store, nil)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer first.Release()

	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := AcquireCodexSessionWriterLock(ctx, store, func() { close(waiting) })
		done <- err
	}()

	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("second writer never reported waiting")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wait error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled writer did not stop waiting")
	}
}

func TestCodexSessionWriterLockDoesNotSerializeDifferentStores(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	root := t.TempDir()

	first, err := AcquireCodexSessionWriterLock(context.Background(), filepath.Join(root, "one"), nil)
	if err != nil {
		t.Fatalf("acquire first store: %v", err)
	}
	defer first.Release()

	second, err := AcquireCodexSessionWriterLock(context.Background(), filepath.Join(root, "two"), func() {
		t.Fatal("different stores must not contend")
	})
	if err != nil {
		t.Fatalf("acquire second store: %v", err)
	}
	second.Release()
}
