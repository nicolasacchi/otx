package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Roles and scope assignment (/api/access/v1)",
}

var (
	roleAssignUser   string
	roleAssignFile   string
	scopeGrantTarget string
)

// -------- role --------

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List roles",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/access/v1/roles", nil)
		if err != nil {
			return err
		}
		return printData("role.list", flattenItems(body))
	},
}

var roleAssignCmd = &cobra.Command{
	Use:   "assign <role-id>",
	Short: "Assign a role to a user (--user)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if roleAssignUser == "" && roleAssignFile == "" {
			return validationErr("--user (single user id) or --file (custom body) is required")
		}
		var body any
		if roleAssignFile != "" {
			b, err := readJSONFile(roleAssignFile)
			if err != nil {
				return err
			}
			body = b
		} else {
			body = map[string]any{"userId": roleAssignUser}
		}
		resp, err := c.Post(context.Background(),
			"/api/access/v1/roles/"+url.PathEscape(args[0])+"/assignments", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var roleRevokeCmd = &cobra.Command{
	Use:   "revoke <role-id> <assignment-id>",
	Short: "Revoke a role assignment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(),
			"/api/access/v1/roles/"+url.PathEscape(args[0])+"/assignments/"+url.PathEscape(args[1])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"revoked": true})
	},
}

// -------- scope --------

var scopeCmd = &cobra.Command{
	Use:   "scope",
	Short: "OAuth scopes available on the tenant + grant/revoke on roles",
}

var scopeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all OAuth scopes the tenant exposes",
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

var scopeGrantCmd = &cobra.Command{
	Use:   "grant <role-id> <scope>",
	Short: "Grant a scope to a role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(),
			"/api/access/v1/roles/"+url.PathEscape(args[0])+"/scopes",
			map[string]any{"scope": args[1]})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var scopeRevokeCmd = &cobra.Command{
	Use:   "revoke <role-id> <scope>",
	Short: "Revoke a scope from a role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(),
			"/api/access/v1/roles/"+url.PathEscape(args[0])+"/scopes/"+url.PathEscape(args[1])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"revoked": true})
	},
}

func init() {
	roleAssignCmd.Flags().StringVar(&roleAssignUser, "user", "", "User ID to assign the role to")
	roleAssignCmd.Flags().StringVar(&roleAssignFile, "file", "", "Path to JSON body (or '-') — overrides --user")
	scopeGrantCmd.Flags().StringVar(&scopeGrantTarget, "target", "", "Optional explicit target (role ID)")

	scopeCmd.AddCommand(scopeListCmd, scopeGrantCmd, scopeRevokeCmd)
	roleCmd.AddCommand(roleListCmd, roleAssignCmd, roleRevokeCmd, scopeCmd)
	rootCmd.AddCommand(roleCmd)
}
