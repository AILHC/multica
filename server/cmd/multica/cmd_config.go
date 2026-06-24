package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration for multica",
	RunE:  runConfigShow,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current CLI configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a CLI configuration value",
	Long:  "Supported keys: server_url, app_url, workspace_id, daemon.device_name, daemon.workspaces_root, daemon.codex_home",
	Args:  exactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	profile := resolveProfile(cmd)
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return err
	}
	printConfig(cmd.OutOrStdout(), profile, cfg)
	return nil
}

func printConfig(w io.Writer, profile string, cfg cli.CLIConfig) {
	path, _ := cli.CLIConfigPathForProfile(profile)
	fmt.Fprintf(w, "Config file: %s\n", path)
	if profile != "" {
		fmt.Fprintf(w, "Profile:      %s\n", profile)
	}
	fmt.Fprintf(w, "server_url:   %s\n", valueOrDefault(cfg.ServerURL, "(not set)"))
	fmt.Fprintf(w, "app_url:      %s\n", valueOrDefault(cfg.AppURL, "(not set)"))
	fmt.Fprintf(w, "workspace_id: %s\n", valueOrDefault(cfg.WorkspaceID, "(not set)"))
	daemonCfg := cfg.Daemon
	if daemonCfg == nil {
		daemonCfg = &cli.DaemonConfig{}
	}
	fmt.Fprintf(w, "daemon.device_name:     %s\n", valueOrDefault(daemonCfg.DeviceName, "(not set)"))
	fmt.Fprintf(w, "daemon.workspaces_root: %s\n", valueOrDefault(daemonCfg.WorkspacesRoot, "(not set)"))
	fmt.Fprintf(w, "daemon.codex_home:      %s\n", valueOrDefault(daemonCfg.CodexHome, "(not set)"))
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	profile := resolveProfile(cmd)
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return err
	}

	if err := setConfigValue(&cfg, key, value); err != nil {
		return err
	}

	if err := cli.SaveCLIConfigForProfile(cfg, profile); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Set %s = %s\n", key, value)
	return nil
}

func setConfigValue(cfg *cli.CLIConfig, key, value string) error {
	switch key {
	case "server_url":
		cfg.ServerURL = value
	case "app_url":
		cfg.AppURL = value
	case "workspace_id":
		cfg.WorkspaceID = value
	case "daemon.device_name":
		ensureDaemonConfig(cfg).DeviceName = value
	case "daemon.workspaces_root":
		ensureDaemonConfig(cfg).WorkspacesRoot = value
	case "daemon.codex_home":
		ensureDaemonConfig(cfg).CodexHome = value
	default:
		return fmt.Errorf("unknown config key %q (supported: server_url, app_url, workspace_id, daemon.device_name, daemon.workspaces_root, daemon.codex_home)", key)
	}
	return nil
}

func ensureDaemonConfig(cfg *cli.CLIConfig) *cli.DaemonConfig {
	if cfg.Daemon == nil {
		cfg.Daemon = &cli.DaemonConfig{}
	}
	return cfg.Daemon
}

func valueOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
