package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/nicolasacchi/otx/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage OneTrust tenant credentials and the local profile file",
}

var (
	cfgAddClientID     string
	cfgAddClientSecret string
	cfgAddBaseURL      string
	cfgAddWriteID      string
	cfgAddWriteSecret  string
)

var configAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or replace a named project profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgAddClientID == "" || cfgAddClientSecret == "" {
			return validationErr("--client-id and --client-secret are required")
		}
		if cfgAddBaseURL == "" {
			cfgAddBaseURL = "https://app-eu.onetrust.com"
		}
		p := &config.Project{
			ClientID:          cfgAddClientID,
			ClientSecret:      cfgAddClientSecret,
			BaseURL:           cfgAddBaseURL,
			WriteClientID:     cfgAddWriteID,
			WriteClientSecret: cfgAddWriteSecret,
		}
		if err := config.AddProject(args[0], p); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "added project %q at %s\n", args[0], config.ConfigPath())
		return nil
	},
}

var configRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a named project profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return config.RemoveProject(args[0])
	},
}

var configUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the default project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return config.SetDefaultProject(args[0])
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured projects (secrets masked)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ListProjects()
		if err != nil {
			return &client.APIError{Kind: "not_found", Detail: "no config file at " + config.ConfigPath(), Hint: "run 'otx config add <name>' to create one"}
		}
		rows := make([]map[string]any, 0, len(cfg.Projects))
		for name, p := range cfg.Projects {
			rows = append(rows, map[string]any{
				"name":      name,
				"client_id": config.MaskSecret(p.ClientID),
				"write":     p.WriteClientID != "",
				"base_url":  p.BaseURL,
				"default":   name == cfg.DefaultProject,
			})
		}
		body, _ := json.Marshal(rows)
		return printData("config.list", body)
	},
}

var configCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active project and its config path",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ListProjects()
		if err != nil {
			return &client.APIError{Kind: "not_found", Detail: "no config file at " + config.ConfigPath()}
		}
		name := projectFlag
		if name == "" {
			name = cfg.DefaultProject
		}
		p := cfg.Projects[name]
		out := map[string]any{
			"path":            config.ConfigPath(),
			"project":         name,
			"default_project": cfg.DefaultProject,
		}
		if p != nil {
			out["base_url"] = p.BaseURL
			out["client_id"] = config.MaskSecret(p.ClientID)
			out["has_write_credentials"] = p.WriteClientID != ""
		}
		return printJSONValue(out)
	},
}

var configDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Probe the configured tenant: OAuth token exchange + scope discovery",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, creds, err := getClient()
		if err != nil {
			return err
		}
		report := map[string]any{
			"project":  creds.ProjectName,
			"base_url": creds.BaseURL,
		}

		// Token exchange.
		tokInfo, err := c.Tokens().PeekToken(context.Background())
		if err != nil {
			report["oauth_token"] = map[string]any{"ok": false, "error": err.Error()}
			return printJSONValue(report)
		}
		report["oauth_token"] = map[string]any{
			"ok":                 true,
			"expires_in_seconds": tokInfo.ExpiresIn,
			"expires_at":         tokInfo.ExpiresAt,
		}

		// Scope discovery — best-effort.
		scopes, scopeErr := c.Get(context.Background(), "/api/access/v1/oauth/scopes", nil)
		if scopeErr != nil {
			report["scopes"] = map[string]any{"ok": false, "error": scopeErr.Error()}
		} else {
			report["scopes"] = map[string]any{"ok": true, "raw": json.RawMessage(scopes)}
		}

		return printJSONValue(report)
	},
}

func init() {
	configAddCmd.Flags().StringVar(&cfgAddClientID, "client-id", "", "OAuth client_id (required)")
	configAddCmd.Flags().StringVar(&cfgAddClientSecret, "client-secret", "", "OAuth client_secret (required)")
	configAddCmd.Flags().StringVar(&cfgAddBaseURL, "base-url", "", "Tenant origin (default https://app-eu.onetrust.com)")
	configAddCmd.Flags().StringVar(&cfgAddWriteID, "write-client-id", "", "Write-scoped OAuth client_id (optional)")
	configAddCmd.Flags().StringVar(&cfgAddWriteSecret, "write-client-secret", "", "Write-scoped OAuth client_secret (optional)")

	configCmd.AddCommand(configAddCmd, configRemoveCmd, configUseCmd, configListCmd, configCurrentCmd, configDoctorCmd)
	rootCmd.AddCommand(configCmd)
}
