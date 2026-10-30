// Copyright 2026 Multica. All rights reserved.

package daemon

import (
	"fmt"
	"github.com/multica-ai/multica/server/internal/cli"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// TestMain redirects the home environment variables to one scratch directory
// for the whole binary before any test runs. Config paths resolve through
// os.UserHomeDir — USERPROFILE on Windows, HOME elsewhere — while Hermes uses
// LOCALAPPDATA on Windows. Process-wide redirection isolates tests that forget
// their own redirect; per-test t.Setenv overrides still take precedence.
func TestMain(m *testing.M) {
	if testutil.RunPiCleanupFixture() {
		os.Exit(0)
	}
	home, err := os.MkdirTemp("", "multica-daemon-tests-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create package test home: %v\n", err)
		os.Exit(1)
	}
	for key, value := range map[string]string{
		"HOME":         home,
		"LOCALAPPDATA": filepath.Join(home, "AppData", "Local"),
		"USERPROFILE":  home,
		"HOMEDRIVE":    filepath.VolumeName(home),
		"HOMEPATH":     strings.TrimPrefix(home, filepath.VolumeName(home)),
	} {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "set package test home %s: %v\n", key, err)
			os.Exit(1)
		}
	}
	for _, key := range []string{
		"MULTICA_AGENT_ID",
		"MULTICA_TASK_ID",
		"MULTICA_DAEMON_PORT",
		"MULTICA_WORKSPACE_ID",
		"MULTICA_APP_URL",
		"MULTICA_PROFILE",
		"MULTICA_SERVER_URL",
		"MULTICA_TASK_CONFIG_ROOT",
		"MULTICA_TOKEN",
	} {
		if err := os.Unsetenv(key); err != nil {
			fmt.Fprintf(os.Stderr, "unset package test environment %s: %v\n", key, err)
			os.Exit(1)
		}
	}

	code := m.Run()
	if err := os.RemoveAll(home); err != nil {
		fmt.Fprintf(os.Stderr, "remove package test home: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

var redirectTestHome = testutil.RedirectUserHome

func TestPackageTestHomeIsIsolated(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	configPath, err := cli.CLIConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	allowedRoot := home
	if isolationRoot := os.Getenv("MULTICA_TEST_ISOLATION_ROOT"); isolationRoot != "" {
		allowedRoot = isolationRoot
	}
	relative, err := filepath.Rel(allowedRoot, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("CLI config path escaped package test home: %s", configPath)
	}
}
