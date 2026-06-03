package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "SCIM 2.0 user management (/api/scim/v2/Users)",
}

var (
	userListFilter  string
	userCreateFile  string
	userPatchFile   string
	groupListFilter string
	groupAddUserIDs string
)

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users (SCIM startIndex/count pagination)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("filter", userListFilter)
		limit, _ := resolvedLimit()
		items, err := c.PaginateSCIM(context.Background(), "/api/scim/v2/Users", params, scimPaginateOpts(limit))
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("user.list", body)
	},
}

var userGetCmd = &cobra.Command{
	Use:   "get <user-id>",
	Short: "Get a user by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/scim/v2/Users/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a SCIM user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(userCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/scim/v2/Users", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var userUpdateCmd = &cobra.Command{
	Use:   "update <user-id>",
	Short: "Patch a user (SCIM patch operations)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(userPatchFile)
		if err != nil {
			return err
		}
		resp, err := c.Patch(context.Background(), "/api/scim/v2/Users/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <user-id>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/scim/v2/Users/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

// -------- group --------

var userGroupCmd = &cobra.Command{
	Use:   "group",
	Short: "SCIM groups",
}

var userGroupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("filter", groupListFilter)
		limit, _ := resolvedLimit()
		items, err := c.PaginateSCIM(context.Background(), "/api/scim/v2/Groups", params, scimPaginateOpts(limit))
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("user.group.list", body)
	},
}

var userGroupGetCmd = &cobra.Command{
	Use:   "get <group-id>",
	Short: "Get a group by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/scim/v2/Groups/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var userGroupAddCmd = &cobra.Command{
	Use:   "add-user <group-id>",
	Short: "Add one or more users to a group (--user-ids, comma-separated)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if groupAddUserIDs == "" {
			return validationErr("--user-ids is required")
		}
		ids := splitCSV(groupAddUserIDs)
		// SCIM PATCH op
		body := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "add",
					"path":  "members",
					"value": idsToMembers(ids),
				},
			},
		}
		resp, err := c.Patch(context.Background(), "/api/scim/v2/Groups/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var userGroupRemoveCmd = &cobra.Command{
	Use:   "remove-user <group-id>",
	Short: "Remove one or more users from a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if groupAddUserIDs == "" {
			return validationErr("--user-ids is required")
		}
		ids := splitCSV(groupAddUserIDs)
		body := map[string]any{
			"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			"Operations": []map[string]any{
				{
					"op":    "remove",
					"path":  "members",
					"value": idsToMembers(ids),
				},
			},
		}
		resp, err := c.Patch(context.Background(), "/api/scim/v2/Groups/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- schema --------

var userSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "SCIM schema discovery (/api/scim/v2/Schemas, /ResourceTypes)",
}

var userSchemaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported SCIM schemas",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/scim/v2/Schemas", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var userSchemaGetCmd = &cobra.Command{
	Use:   "get <schema-name>",
	Short: "Get a specific schema",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/scim/v2/Schemas/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var userResourceTypesCmd = &cobra.Command{
	Use:   "resource-types",
	Short: "List supported SCIM ResourceTypes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/scim/v2/ResourceTypes", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	userListCmd.Flags().StringVar(&userListFilter, "filter", "", "SCIM filter expression (e.g. 'userName eq \"alice\"')")
	userCreateCmd.Flags().StringVar(&userCreateFile, "file", "", "Path to SCIM user JSON (or '-')")
	userUpdateCmd.Flags().StringVar(&userPatchFile, "file", "", "Path to SCIM patch JSON (or '-')")
	userGroupListCmd.Flags().StringVar(&groupListFilter, "filter", "", "SCIM filter expression")
	userGroupAddCmd.Flags().StringVar(&groupAddUserIDs, "user-ids", "", "Comma-separated user IDs to add")
	userGroupRemoveCmd.Flags().StringVar(&groupAddUserIDs, "user-ids", "", "Comma-separated user IDs to remove")

	userGroupCmd.AddCommand(userGroupListCmd, userGroupGetCmd, userGroupAddCmd, userGroupRemoveCmd)
	userSchemaCmd.AddCommand(userSchemaListCmd, userSchemaGetCmd, userResourceTypesCmd)
	userCmd.AddCommand(userListCmd, userGetCmd, userCreateCmd, userUpdateCmd, userDeleteCmd, userGroupCmd, userSchemaCmd)
	rootCmd.AddCommand(userCmd)
}

// idsToMembers converts a list of user IDs to the SCIM members value shape.
func idsToMembers(ids []string) []map[string]any {
	out := make([]map[string]any, len(ids))
	for i, id := range ids {
		out[i] = map[string]any{"value": id}
	}
	return out
}

func splitCSV(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		if r == ' ' && cur == "" {
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
