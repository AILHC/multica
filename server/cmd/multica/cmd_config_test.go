package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func testCommandWithProfileFlag(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("profile", "", "")
	return cmd
}

func TestConfigSetDaemonKeys(t *testing.T) {
	testHome(t)

	cmd := testCommandWithProfileFlag(t)
	if err := runConfigSet(cmd, []string{"daemon.device_name", "home-pc"}); err != nil {
		t.Fatalf("config set daemon.device_name: %v", err)
	}
	if err := runConfigSet(cmd, []string{"daemon.workspaces_root", "F:\\ai-runtime\\multica\\workspaces"}); err != nil {
		t.Fatalf("config set daemon.workspaces_root: %v", err)
	}
	if err := runConfigSet(cmd, []string{"daemon.codex_home", "F:\\ai-runtime\\multica\\codex-home"}); err != nil {
		t.Fatalf("config set daemon.codex_home: %v", err)
	}

	cfg, err := cli.LoadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Daemon == nil {
		t.Fatalf("daemon config is nil")
	}
	if cfg.Daemon.DeviceName != "home-pc" {
		t.Fatalf("daemon.device_name = %q, want home-pc", cfg.Daemon.DeviceName)
	}
	if cfg.Daemon.WorkspacesRoot == "" || cfg.Daemon.CodexHome == "" {
		t.Fatalf("daemon config missing paths: %+v", cfg.Daemon)
	}
}

func TestConfigShowIncludesDaemonKeysButNotToken(t *testing.T) {
	testHome(t)
	if err := cli.SaveCLIConfig(cli.CLIConfig{
		Token: "mul_secret",
		Daemon: &cli.DaemonConfig{
			DeviceName:     "home-pc",
			WorkspacesRoot: "F:\\ai-runtime\\multica\\workspaces",
			CodexHome:      "F:\\ai-runtime\\multica\\codex-home",
		},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := cli.LoadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	printConfig(&out, "", cfg)

	got := out.String()
	for _, want := range []string{"daemon.device_name:", "daemon.workspaces_root:", "daemon.codex_home:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("config show = %q, want %s", got, want)
		}
	}
	if strings.Contains(got, "mul_secret") {
		t.Fatalf("config show leaked token: %q", got)
	}
}
