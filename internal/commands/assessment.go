package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var assessmentCmd = &cobra.Command{
	Use:   "assessment",
	Short: "Assessment lifecycle (PIA/DPIA/TIA/TPRM questionnaires/etc — workflow Draft→Under Review→Completed)",
}

var (
	assessLaunchFile   string
	assessListApprover string
	assessListRespond  string
	assessListTemplate string
	assessListStage    string
	assessLinkFile     string
	assessRiskFile     string
	assessAttachFile   string
	assessAttachPath   string
)

var assessListCmd = &cobra.Command{
	Use:   "list",
	Short: "List assessments (filter by approver/respondent/template/stage)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith(
			"approverIds", assessListApprover,
			"respondentIds", assessListRespond,
			"templateIds", assessListTemplate,
			"stage", assessListStage,
		)
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/assessment/v2/assessments", params, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("assessment.list", body)
	},
}

var assessGetCmd = &cobra.Command{
	Use:   "get <assessment-id>",
	Short: "Get an assessment by ID",
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

var assessLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch an assessment (POST /api/assessment/v2/assessments)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(assessLaunchFile)
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

var assessExportCmd = &cobra.Command{
	Use:   "export <assessment-id>",
	Short: "Export an assessment (returns the response body verbatim)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/export", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- workflow --------

var assessWorkflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Assessment workflow stage transitions",
}

var assessWFGetCmd = &cobra.Command{
	Use:   "get <assessment-id>",
	Short: "Get workflow stages for an assessment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/workflows", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var assessSubmitCmd = &cobra.Command{
	Use:   "submit <assessment-id>",
	Short: "Submit assessment for review (Draft → Under Review)",
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

var assessCompleteCmd = &cobra.Command{
	Use:   "complete <assessment-id>",
	Short: "Complete review (Under Review → Completed)",
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

// -------- result options --------

var assessResultCmd = &cobra.Command{
	Use:   "result",
	Short: "Assessment result options",
}

var assessResultListCmd = &cobra.Command{
	Use:   "list <assessment-id>",
	Short: "List allowed result options",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/results", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- linked-assessments --------

var assessLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link / list linked assessments",
}

var assessLinkListCmd = &cobra.Command{
	Use:   "list <assessment-id>",
	Short: "List linked assessments",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/linked-assessments", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var assessLinkAddCmd = &cobra.Command{
	Use:   "add <assessment-id>",
	Short: "Add a manual link to another assessment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(assessLinkFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/links", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- risk on assessment --------

var assessRiskCmd = &cobra.Command{
	Use:   "risk",
	Short: "Risks attached to an assessment",
}

var assessRiskListCmd = &cobra.Command{
	Use:   "list <assessment-id>",
	Short: "List risks on this assessment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/risks", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var assessRiskCreateCmd = &cobra.Command{
	Use:   "create <assessment-id>",
	Short: "Create a risk on this assessment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(assessRiskFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/risks", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- attachment --------

var assessAttachCmd = &cobra.Command{
	Use:   "attachment",
	Short: "Attach a file (PDF/etc) to an assessment",
}

var assessAttachUploadCmd = &cobra.Command{
	Use:   "upload <assessment-id>",
	Short: "Upload an attachment (max 64MB)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if assessAttachPath == "" {
			return validationErr("--file is required")
		}
		body, contentType, err := buildMultipart(assessAttachPath)
		if err != nil {
			return err
		}
		resp, err := c.Raw(context.Background(), "POST",
			"/api/assessment/v2/assessments/"+url.PathEscape(args[0])+"/attachments",
			contentType, body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	assessListCmd.Flags().StringVar(&assessListApprover, "approver", "", "Filter by approver IDs (comma-separated)")
	assessListCmd.Flags().StringVar(&assessListRespond, "respondent", "", "Filter by respondent IDs")
	assessListCmd.Flags().StringVar(&assessListTemplate, "template", "", "Filter by template IDs")
	assessListCmd.Flags().StringVar(&assessListStage, "stage", "", "Filter by stage (DRAFT/UNDER_REVIEW/COMPLETED)")
	assessLaunchCmd.Flags().StringVar(&assessLaunchFile, "file", "", "Path to JSON launch body (or '-')")
	assessLinkAddCmd.Flags().StringVar(&assessLinkFile, "file", "", "Path to JSON link body (or '-')")
	assessRiskCreateCmd.Flags().StringVar(&assessRiskFile, "file", "", "Path to JSON risk body (or '-')")
	assessAttachUploadCmd.Flags().StringVar(&assessAttachPath, "file", "", "Path to file to upload")

	assessWorkflowCmd.AddCommand(assessWFGetCmd, assessSubmitCmd, assessCompleteCmd)
	assessResultCmd.AddCommand(assessResultListCmd)
	assessLinkCmd.AddCommand(assessLinkListCmd, assessLinkAddCmd)
	assessRiskCmd.AddCommand(assessRiskListCmd, assessRiskCreateCmd)
	assessAttachCmd.AddCommand(assessAttachUploadCmd)

	assessmentCmd.AddCommand(
		assessListCmd,
		assessGetCmd,
		assessLaunchCmd,
		assessExportCmd,
		assessWorkflowCmd,
		assessResultCmd,
		assessLinkCmd,
		assessRiskCmd,
		assessAttachCmd,
	)
	rootCmd.AddCommand(assessmentCmd)
}
