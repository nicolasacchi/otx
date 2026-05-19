package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Organizations (multi-tenant hierarchy, data segregation)",
}

var orgCreateFile string

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/external/organizations", nil)
		if err != nil {
			return err
		}
		return printData("org.list", flattenItems(body))
	},
}

var orgGetCmd = &cobra.Command{
	Use:   "get <org-id>",
	Short: "Get an organization by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/external/organizations/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var orgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(orgCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/external/organizations", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var orgDeleteCmd = &cobra.Command{
	Use:   "delete <org-id>",
	Short: "Delete an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/external/organizations/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

func init() {
	orgCreateCmd.Flags().StringVar(&orgCreateFile, "file", "", "Path to JSON body (or '-')")
	orgCmd.AddCommand(orgListCmd, orgGetCmd, orgCreateCmd, orgDeleteCmd)
	rootCmd.AddCommand(orgCmd)
}
