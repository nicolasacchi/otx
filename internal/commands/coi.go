package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
)

var coiCmd = &cobra.Command{
	Use:   "coi",
	Short: "Conflicts of Interest / Disclosure Management (limited public API)",
}

// -------- disclosure --------

var coiDisclosureCmd = &cobra.Command{
	Use:   "disclosure",
	Short: "CoI disclosures (best-effort coverage)",
}

var (
	coiSubmitFile string
)

var coiDisclosureSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a disclosure (best-effort)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(coiSubmitFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/disclosure/v1/disclosures", body)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("coi disclosure submit")
			}
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var coiDisclosureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List disclosures (best-effort)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/disclosure/v1/disclosures", nil)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("coi disclosure list")
			}
			return err
		}
		return printData("coi.disclosure.list", flattenItems(body))
	},
}

var coiDisclosureGetCmd = &cobra.Command{
	Use:   "get <disclosure-id>",
	Short: "Get a disclosure (best-effort)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/disclosure/v1/disclosures/"+url.PathEscape(args[0]), nil)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("coi disclosure get")
			}
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- approval --------

var coiApprovalCmd = &cobra.Command{
	Use:   "approval",
	Short: "CoI approval workflow (best-effort)",
}

var coiApprovalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending CoI approvals (best-effort)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/disclosure/v1/approvals", nil)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("coi approval list")
			}
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var coiApprovalApproveCmd = &cobra.Command{
	Use:   "approve <approval-id>",
	Short: "Approve a CoI disclosure (best-effort)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(),
			"/api/disclosure/v1/approvals/"+url.PathEscape(args[0])+"/approve", nil)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return notPubliclyDocumented("coi approval approve")
			}
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	coiDisclosureSubmitCmd.Flags().StringVar(&coiSubmitFile, "file", "", "Path to JSON body")
	coiDisclosureCmd.AddCommand(coiDisclosureSubmitCmd, coiDisclosureListCmd, coiDisclosureGetCmd)
	coiApprovalCmd.AddCommand(coiApprovalListCmd, coiApprovalApproveCmd)
	coiCmd.AddCommand(coiDisclosureCmd, coiApprovalCmd)
	rootCmd.AddCommand(coiCmd)
}
