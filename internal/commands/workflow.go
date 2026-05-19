package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Workflow definitions, tasks, and approvals (cross-product automation)",
}

// -------- workflow definitions --------

var wfDefCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Workflow definitions",
}

var wfDefListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflow definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/workflow/v1/workflows", nil, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("workflow.workflow.list", body)
	},
}

var wfDefGetCmd = &cobra.Command{
	Use:   "get <workflow-id>",
	Short: "Get a workflow definition by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/workflow/v1/workflows/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var wfCreateFile string

var wfDefCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workflow definition",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(wfCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/workflow/v1/workflows", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var wfDefExportCmd = &cobra.Command{
	Use:   "export <workflow-id>",
	Short: "Export a workflow definition as JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Post(context.Background(), "/api/workflow/v1/workflows/"+url.PathEscape(args[0])+"/export", map[string]any{})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var wfImportFile string

var wfDefImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a workflow definition from a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(wfImportFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/workflow/v1/workflows/import", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- task --------

var wfTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Workflow tasks",
}

var (
	taskCreateFile string
	taskAssignee   string
)

var wfTaskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("assigneeId", taskAssignee)
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/workflow/v1/tasks", params, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("workflow.task.list", body)
	},
}

var wfTaskCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(taskCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/workflow/v1/tasks", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var wfTaskCompleteCmd = &cobra.Command{
	Use:   "complete <task-id>",
	Short: "Mark a task complete",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Patch(context.Background(),
			"/api/workflow/v1/tasks/"+url.PathEscape(args[0]),
			map[string]any{"status": "COMPLETED"})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- approval --------

var wfApprovalCmd = &cobra.Command{
	Use:   "approval",
	Short: "Approval requests",
}

var wfApprovalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending approvals",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/workflow/v1/approvals", nil)
		if err != nil {
			return err
		}
		return printData("workflow.approval.list", flattenItems(body))
	},
}

var wfApprovalApproveCmd = &cobra.Command{
	Use:   "approve <approval-id>",
	Short: "Approve a pending request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(),
			"/api/workflow/v1/approvals/"+url.PathEscape(args[0])+"/approve", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var wfApprovalRejectCmd = &cobra.Command{
	Use:   "reject <approval-id>",
	Short: "Reject a pending request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(),
			"/api/workflow/v1/approvals/"+url.PathEscape(args[0])+"/reject", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	wfDefCreateCmd.Flags().StringVar(&wfCreateFile, "file", "", "Path to JSON body (or '-')")
	wfDefImportCmd.Flags().StringVar(&wfImportFile, "file", "", "Path to JSON definition (or '-')")
	wfTaskCreateCmd.Flags().StringVar(&taskCreateFile, "file", "", "Path to JSON body (or '-')")
	wfTaskListCmd.Flags().StringVar(&taskAssignee, "assignee", "", "Filter by assignee ID")

	wfDefCmd.AddCommand(wfDefListCmd, wfDefGetCmd, wfDefCreateCmd, wfDefExportCmd, wfDefImportCmd)
	wfTaskCmd.AddCommand(wfTaskListCmd, wfTaskCreateCmd, wfTaskCompleteCmd)
	wfApprovalCmd.AddCommand(wfApprovalListCmd, wfApprovalApproveCmd, wfApprovalRejectCmd)
	workflowCmd.AddCommand(wfDefCmd, wfTaskCmd, wfApprovalCmd)
	rootCmd.AddCommand(workflowCmd)
}
