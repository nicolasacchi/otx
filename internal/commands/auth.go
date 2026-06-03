package commands

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Inspect OAuth tokens and OneTrust scopes",
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "OAuth bearer token operations",
}

var authTokenGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Fetch (and cache) a bearer token for the active tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		info, err := c.Tokens().PeekToken(context.Background())
		if err != nil {
			return err
		}
		if c.Tokens().IsStatic() {
			return printJSONValue(map[string]any{
				"access_token": info.AccessToken,
				"mode":         "api_key",
				"note":         "static API key — no OAuth expiry",
			})
		}
		return printJSONValue(map[string]any{
			"access_token":       info.AccessToken,
			"expires_at":         info.ExpiresAt,
			"expires_in_seconds": info.ExpiresIn,
		})
	},
}

var authTokenRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Invalidate the cached token and fetch a fresh one",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		c.Tokens().Invalidate()
		info, err := c.Tokens().PeekToken(context.Background())
		if err != nil {
			return err
		}
		return printJSONValue(map[string]any{
			"refreshed":          true,
			"access_token":       info.AccessToken,
			"expires_in_seconds": info.ExpiresIn,
		})
	},
}

var authTokenValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Probe a token-gated endpoint to confirm the cached bearer works",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, creds, err := getClient()
		if err != nil {
			return err
		}
		// Hitting /api/access/v1/oauth/scopes is the cheapest auth-only probe.
		body, err := c.Get(context.Background(), "/api/access/v1/oauth/scopes", nil)
		if err != nil {
			// A non-401 error means the bearer was accepted — the endpoint is
			// just absent/forbidden — so the token is still valid. Only a 401
			// (auth_failed) indicates a genuinely bad token.
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.Kind != "auth_failed" {
				return printJSONValue(map[string]any{
					"ok":             true,
					"project":        creds.ProjectName,
					"base_url":       creds.BaseURL,
					"token_accepted": true,
					"note":           "token is valid (request authenticated); scope-discovery endpoint is not available on this tenant",
				})
			}
			return err
		}
		return printJSONValue(map[string]any{
			"ok":       true,
			"project":  creds.ProjectName,
			"base_url": creds.BaseURL,
			"raw":      json.RawMessage(body),
		})
	},
}

var authScopesCmd = &cobra.Command{
	Use:   "scopes",
	Short: "Inspect OAuth scopes",
}

var authScopesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List OAuth scopes available to the active credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/access/v1/oauth/scopes", nil)
		if err != nil {
			if scopesUnavailable(err) {
				return renderScopesUnavailable()
			}
			return err
		}
		return printData("auth.scopes.list", flattenItems(body))
	},
}

var authScopesCheckCmd = &cobra.Command{
	Use:   "check <scope>",
	Short: "Check whether a specific scope is granted",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/access/v1/oauth/scopes", nil)
		if err != nil {
			if scopesUnavailable(err) {
				return printJSONValue(map[string]any{
					"scope":   args[0],
					"granted": "unknown",
					"note":    "scope-discovery endpoint not available on this tenant — verify in the OneTrust UI",
				})
			}
			return err
		}
		want := args[0]
		var parsed []map[string]any
		_ = json.Unmarshal(flattenItems(body), &parsed)
		found := false
		for _, s := range parsed {
			if v, ok := s["scope"].(string); ok && v == want {
				found = true
				break
			}
			if v, ok := s["name"].(string); ok && v == want {
				found = true
				break
			}
		}
		return printJSONValue(map[string]any{
			"scope":   want,
			"granted": found,
		})
	},
}

func init() {
	authTokenCmd.AddCommand(authTokenGetCmd, authTokenRefreshCmd, authTokenValidateCmd)
	authScopesCmd.AddCommand(authScopesListCmd, authScopesCheckCmd)
	authCmd.AddCommand(authTokenCmd, authScopesCmd)
	rootCmd.AddCommand(authCmd)
}
