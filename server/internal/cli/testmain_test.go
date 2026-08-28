package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "multica-cli-tests-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create package test home: %v\n", err)
		os.Exit(1)
	}
	for key, value := range map[string]string{
		"HOME":        home,
		"USERPROFILE": home,
		"HOMEDRIVE":   filepath.VolumeName(home),
		"HOMEPATH":    strings.TrimPrefix(home, filepath.VolumeName(home)),
	} {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "set package test home %s: %v\n", key, err)
			os.Exit(1)
		}
	}
	for _, key := range []string{
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

func TestPackageTestHomeIsIsolated(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	configPath, err := CLIConfigPath()
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
