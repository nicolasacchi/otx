package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "OAuth client-credential and (legacy) API-key management",
}

// -------- client --------

var credClientCmd = &cobra.Command{
	Use:   "client",
	Short: "OAuth client credentials",
}

var credClientCreateFile string

var credClientListCmd = &cobra.Command{
	Use:   "list",
	Short: "List OAuth client credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/access/v1/oauth/credentials/client", nil)
		if err != nil {
			return err
		}
		return printData("credential.client.list", flattenItems(body))
	},
}

var credClientCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new OAuth client credential pair",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(credClientCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/access/v1/oauth/credentials/client", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var credClientDeleteCmd = &cobra.Command{
	Use:   "delete <client-id>",
	Short: "Delete an OAuth client credential",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/access/v1/oauth/credentials/client/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

// -------- api-key (legacy) --------

var credAPIKeyCmd = &cobra.Command{
	Use:   "api-key",
	Short: "Legacy API keys — OneTrust recommends OAuth client credentials instead",
}

var credAPIKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List legacy API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "⚠  Legacy API keys are deprecated — prefer OAuth client credentials.")
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/access/v1/api-keys", nil)
		if err != nil {
			return err
		}
		return printData("credential.api-key.list", flattenItems(body))
	},
}

var credAPIKeyCreateFile string

var credAPIKeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a legacy API key (emits a deprecation warning)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "⚠  Legacy API keys are deprecated — prefer OAuth client credentials.")
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(credAPIKeyCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/access/v1/api-keys", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var credAPIKeyDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete a legacy API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/access/v1/api-keys/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

// -------- scope (alias) --------

var credScopeCmd = &cobra.Command{
	Use:   "scope",
	Short: "List OAuth scopes available on this tenant (alias for 'otx auth scopes list')",
}

var credScopeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scopes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/access/v1/oauth/scopes", nil)
		if err != nil {
			return err
		}
		return printData("auth.scopes.list", flattenItems(body))
	},
}

func init() {
	credClientCreateCmd.Flags().StringVar(&credClientCreateFile, "file", "", "Path to JSON body (or '-')")
	credAPIKeyCreateCmd.Flags().StringVar(&credAPIKeyCreateFile, "file", "", "Path to JSON body (or '-')")

	credClientCmd.AddCommand(credClientListCmd, credClientCreateCmd, credClientDeleteCmd)
	credAPIKeyCmd.AddCommand(credAPIKeyListCmd, credAPIKeyCreateCmd, credAPIKeyDeleteCmd)
	credScopeCmd.AddCommand(credScopeListCmd)
	credentialCmd.AddCommand(credClientCmd, credAPIKeyCmd, credScopeCmd)
	rootCmd.AddCommand(credentialCmd)
}
