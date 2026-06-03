package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var tprmCmd = &cobra.Command{
	Use:   "tprm",
	Short: "Third-Party Risk Management — vendor inventory, assessments, questionnaires, risk scores",
}

// -------- vendor --------

var tprmVendorCmd = &cobra.Command{
	Use:   "vendor",
	Short: "Vendor inventory CRUD (/api/inventory/v2/inventories/vendor)",
}

var (
	vendorCreateFile string
	vendorUpdateFile string
	vendorListAll    bool
)

var tprmVendorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List vendors",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		opts := clientPageOpts{Size: 50, MaxItems: limit}
		if vendorListAll {
			opts.MaxItems = 0
		}
		items, err := c.PaginatePage(context.Background(), "/api/inventory/v2/inventories/vendor", nil, opts)
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("tprm.vendor.list", body)
	},
}

var tprmVendorGetCmd = &cobra.Command{
	Use:   "get <vendor-id>",
	Short: "Get a vendor by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/inventories/vendor/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var tprmVendorCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a vendor",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(vendorCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/inventories/vendor", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var tprmVendorUpdateCmd = &cobra.Command{
	Use:   "update <vendor-id>",
	Short: "Update a vendor (PUT)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(vendorUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/inventory/v2/inventories/vendor/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var tprmVendorDeleteCmd = &cobra.Command{
	Use:   "delete <vendor-id>",
	Short: "Delete a vendor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/inventory/v2/inventories/vendor/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

var tprmVendorLinkChildCmd = &cobra.Command{
	Use:   "link-child <parent-id> <child-vendor-id>",
	Short: "Link a vendor as a child of another vendor",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(),
			"/api/inventory/v2/inventories/"+url.PathEscape(args[0])+"/vendors/"+url.PathEscape(args[1]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- assessment (TPRM-specific) --------

var tprmAssessCmd = &cobra.Command{
	Use:   "assessment",
	Short: "TPRM assessments (vendor-scoped wrappers over /api/assessment/v2)",
}

var tprmAssessLaunchFile string

var tprmAssessLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a vendor assessment",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(tprmAssessLaunchFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/assessments", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var (
	tprmAssessVendorID string
	tprmAssessStage    string
)

var tprmAssessListCmd = &cobra.Command{
	Use:   "list",
	Short: "List TPRM assessments (filter by vendor + stage)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("inventoryIds", tprmAssessVendorID, "stage", tprmAssessStage)
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/assessment/v2/assessments", params, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("assessment.list", body)
	},
}

var tprmAssessGetCmd = &cobra.Command{
	Use:   "get <assessment-id>",
	Short: "Get a TPRM assessment by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var tprmAssessSubmitCmd = &cobra.Command{
	Use:   "submit <assessment-id>",
	Short: "Submit assessment (Draft → Under Review)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/submit", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var tprmAssessCompleteCmd = &cobra.Command{
	Use:   "complete <assessment-id>",
	Short: "Complete (Under Review → Completed)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/complete", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- questionnaire --------

var tprmQCmd = &cobra.Command{
	Use:   "questionnaire",
	Short: "Vendor questionnaires (distribution + responses + QRA suggestions)",
}

var tprmQSendFile string

var tprmQSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a questionnaire (launches an assessment from a template)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(tprmQSendFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/assessments", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var tprmQResponsesCmd = &cobra.Command{
	Use:   "responses <assessment-id>",
	Short: "Fetch questionnaire responses",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/responses", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var tprmQQRAFile string

var tprmQQRACmd = &cobra.Command{
	Use:   "qra-suggest",
	Short: "Request AI Questionnaire Response Automation suggestions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(tprmQQRAFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/qra/suggestions", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- score --------

var tprmScoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Vendor risk scores (incl. BitSight / SecurityScorecard surfaces)",
}

var tprmScoreGetCmd = &cobra.Command{
	Use:   "get <vendor-id>",
	Short: "Get a vendor's risk score",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/inventories/vendor/"+url.PathEscape(args[0])+"/risk-score", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	tprmVendorListCmd.Flags().BoolVar(&vendorListAll, "all", false, "Fetch all pages")
	tprmVendorCreateCmd.Flags().StringVar(&vendorCreateFile, "file", "", "Path to JSON body (or '-')")
	tprmVendorUpdateCmd.Flags().StringVar(&vendorUpdateFile, "file", "", "Path to JSON body (or '-')")
	tprmAssessLaunchCmd.Flags().StringVar(&tprmAssessLaunchFile, "file", "", "Path to JSON launch body (or '-')")
	tprmAssessListCmd.Flags().StringVar(&tprmAssessVendorID, "vendor", "", "Filter by vendor inventory ID")
	tprmAssessListCmd.Flags().StringVar(&tprmAssessStage, "stage", "", "Filter by stage")
	tprmQSendCmd.Flags().StringVar(&tprmQSendFile, "file", "", "Path to JSON questionnaire body")
	tprmQQRACmd.Flags().StringVar(&tprmQQRAFile, "file", "", "Path to JSON QRA body")

	tprmVendorCmd.AddCommand(tprmVendorListCmd, tprmVendorGetCmd, tprmVendorCreateCmd, tprmVendorUpdateCmd, tprmVendorDeleteCmd, tprmVendorLinkChildCmd)
	tprmAssessCmd.AddCommand(tprmAssessLaunchCmd, tprmAssessListCmd, tprmAssessGetCmd, tprmAssessSubmitCmd, tprmAssessCompleteCmd)
	tprmQCmd.AddCommand(tprmQSendCmd, tprmQResponsesCmd, tprmQQRACmd)
	tprmScoreCmd.AddCommand(tprmScoreGetCmd)
	tprmCmd.AddCommand(tprmVendorCmd, tprmAssessCmd, tprmQCmd, tprmScoreCmd)
	rootCmd.AddCommand(tprmCmd)
}
