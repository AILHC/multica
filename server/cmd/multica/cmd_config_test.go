package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

func newConfigTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config"}
	cmd.Flags().String("profile", "", "")
	return cmd
}

func TestRunConfigSetPersistsSupportedKeysInProfile(t *testing.T) {
	testHome(t)

	cmd := newConfigTestCmd()
	_ = cmd.Flags().Set("profile", "dev")

	stderr := captureStderr(t)
	defer stderr.restore()
	if err := runConfigSet(cmd, []string{"server_url", "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("runConfigSet server_url: %v", err)
	}
	if err := runConfigSet(cmd, []string{"app_url", "http://127.0.0.1:3000"}); err != nil {
		t.Fatalf("runConfigSet app_url: %v", err)
	}
	if err := runConfigSet(cmd, []string{"workspace_id", "ws-123"}); err != nil {
		t.Fatalf("runConfigSet workspace_id: %v", err)
	}
	_ = stderr.read()

	cfg, err := cli.LoadCLIConfigForProfile("dev")
	if err != nil {
		t.Fatalf("LoadCLIConfigForProfile: %v", err)
	}
	if cfg.ServerURL != "http://127.0.0.1:8080" || cfg.AppURL != "http://127.0.0.1:3000" || cfg.WorkspaceID != "ws-123" {
		t.Fatalf("config = %#v, want persisted supported keys", cfg)
	}
}

func TestRunConfigSetPersistsDaemonKeysInProfile(t *testing.T) {
	testHome(t)

	cmd := newConfigTestCmd()
	_ = cmd.Flags().Set("profile", "dev")

	stderr := captureStderr(t)
	defer stderr.restore()
	if err := runConfigSet(cmd, []string{"daemon.device_name", "home-pc"}); err != nil {
		t.Fatalf("runConfigSet daemon.device_name: %v", err)
	}
	if err := runConfigSet(cmd, []string{"daemon.workspaces_root", "F:\\ai-runtime\\multica\\workspaces"}); err != nil {
		t.Fatalf("runConfigSet daemon.workspaces_root: %v", err)
	}
	if err := runConfigSet(cmd, []string{"daemon.codex_home", "F:\\ai-runtime\\multica\\codex-home"}); err != nil {
		t.Fatalf("runConfigSet daemon.codex_home: %v", err)
	}
	_ = stderr.read()

	cfg, err := cli.LoadCLIConfigForProfile("dev")
	if err != nil {
		t.Fatalf("LoadCLIConfigForProfile: %v", err)
	}
	if cfg.Daemon == nil {
		t.Fatalf("daemon config is nil")
	}
	if cfg.Daemon.DeviceName != "home-pc" {
		t.Fatalf("daemon.device_name = %q, want home-pc", cfg.Daemon.DeviceName)
	}
	if cfg.Daemon.WorkspacesRoot != "F:\\ai-runtime\\multica\\workspaces" {
		t.Fatalf("daemon.workspaces_root = %q", cfg.Daemon.WorkspacesRoot)
	}
	if cfg.Daemon.CodexHome != "F:\\ai-runtime\\multica\\codex-home" {
		t.Fatalf("daemon.codex_home = %q", cfg.Daemon.CodexHome)
	}
}

func TestRunConfigShowIncludesProfileDefaultsAndDaemonKeys(t *testing.T) {
	testHome(t)

	cmd := newConfigTestCmd()
	_ = cmd.Flags().Set("profile", "empty")

	out, err := captureStdout(t, func() error { return runConfigShow(cmd, nil) })
	if err != nil {
		t.Fatalf("runConfigShow: %v", err)
	}
	for _, want := range []string{
		"Profile:      empty",
		"server_url:   (not set)",
		"app_url:      (not set)",
		"workspace_id: (not set)",
		"daemon.device_name:     (not set)",
		"daemon.workspaces_root: (not set)",
		"daemon.codex_home:      (not set)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("runConfigShow output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintConfigIncludesDaemonKeysButNotToken(t *testing.T) {
	testHome(t)

	cfg := cli.CLIConfig{
		Token: "mul_secret",
		Daemon: &cli.DaemonConfig{
			DeviceName:     "home-pc",
			WorkspacesRoot: "F:\\ai-runtime\\multica\\workspaces",
			CodexHome:      "F:\\ai-runtime\\multica\\codex-home",
		},
	}
	var out bytes.Buffer
	printConfig(&out, "", cfg)

	got := out.String()
	for _, want := range []string{"daemon.device_name:", "daemon.workspaces_root:", "daemon.codex_home:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("printConfig output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "mul_secret") {
		t.Fatalf("printConfig leaked token: %q", got)
	}
}

func TestRunConfigSetRejectsUnknownKey(t *testing.T) {
	testHome(t)

	cmd := newConfigTestCmd()
	err := runConfigSet(cmd, []string{"token", "secret"})
	if err == nil || !strings.Contains(err.Error(), "unknown config key") {
		t.Fatalf("runConfigSet error = %v, want unknown key", err)
	}
}
