package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
)

var ethicsCmd = &cobra.Command{
	Use:   "ethics",
	Short: "Ethics hotline / case management (limited public API — primarily UI-driven)",
}

// notPubliclyDocumented returns the standard envelope for an endpoint that
// exists in OneTrust's product but lacks a publicly documented REST surface.
func notPubliclyDocumented(verb string) *client.APIError {
	return &client.APIError{
		Kind:   "not_publicly_documented",
		Detail: verb + ": OneTrust has not published a REST endpoint for this operation",
		Hint:   "this product surface is primarily managed via the OneTrust UI; contact OneTrust support if you have access to private API docs",
	}
}

// -------- case --------

var ethicsCaseCmd = &cobra.Command{
	Use:   "case",
	Short: "Ethics hotline cases (best-effort coverage)",
}

var ethicsCaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cases (best-effort — endpoint inferred)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/case-management/v1/cases", nil)
		if err != nil {
			// If the endpoint isn't exposed publicly, return the standard envelope.
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("ethics case list")
			}
			return err
		}
		return printData("ethics.case.list", flattenItems(body))
	},
}

var ethicsCaseGetCmd = &cobra.Command{
	Use:   "get <case-id>",
	Short: "Get a case (best-effort)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/case-management/v1/cases/"+url.PathEscape(args[0]), nil)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("ethics case get")
			}
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var ethicsCaseUpdateFile string

var ethicsCaseUpdateCmd = &cobra.Command{
	Use:   "update <case-id>",
	Short: "Update a case (best-effort)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(ethicsCaseUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Patch(context.Background(), "/api/case-management/v1/cases/"+url.PathEscape(args[0]), body)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("ethics case update")
			}
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- hotline-config --------

var ethicsHotlineCmd = &cobra.Command{
	Use:   "hotline-config",
	Short: "Hotline channel configuration (call/web/SMS intake; limited public surface)",
}

var ethicsHotlineGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the hotline configuration (best-effort)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/case-management/v1/hotline/config", nil)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("ethics hotline-config get")
			}
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	ethicsCaseUpdateCmd.Flags().StringVar(&ethicsCaseUpdateFile, "file", "", "Path to JSON body")

	ethicsCaseCmd.AddCommand(ethicsCaseListCmd, ethicsCaseGetCmd, ethicsCaseUpdateCmd)
	ethicsHotlineCmd.AddCommand(ethicsHotlineGetCmd)
	ethicsCmd.AddCommand(ethicsCaseCmd, ethicsHotlineCmd)
	rootCmd.AddCommand(ethicsCmd)
}
