package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Internal audit — workpapers, findings, audit plans",
}

// -------- workpaper --------

var auditWPCmd = &cobra.Command{
	Use:   "workpaper",
	Short: "Audit workpapers",
}

var (
	wpListFile   string
	wpCreateFile string
)

var auditWPListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workpapers (POST /api/audit/v2/workpapers — filter body via --file or empty)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body := map[string]any{}
		if wpListFile != "" {
			parsed, err := readJSONFile(wpListFile)
			if err != nil {
				return err
			}
			body = parsed.(map[string]any)
		}
		resp, err := c.Post(context.Background(), "/api/audit/v2/workpapers", body)
		if err != nil {
			return err
		}
		return printData("audit.workpaper.list", flattenItems(json.RawMessage(resp)))
	},
}

var auditWPGetCmd = &cobra.Command{
	Use:   "get <workpaper-id>",
	Short: "Get a workpaper by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/audit/v2/workpapers/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var auditWPCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workpaper",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(wpCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/audit/v2/workpapers", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- finding --------

var auditFindingCmd = &cobra.Command{
	Use:   "finding",
	Short: "Audit findings",
}

var (
	findingCreateFile string
	findingUpdateFile string
)

var auditFindingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List findings",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/audit/v2/findings", nil, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("audit.finding.list", body)
	},
}

var auditFindingGetCmd = &cobra.Command{
	Use:   "get <finding-id>",
	Short: "Get a finding",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/audit/v2/findings/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var auditFindingCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a finding",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(findingCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/audit/v2/findings", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var auditFindingUpdateCmd = &cobra.Command{
	Use:   "update <finding-id>",
	Short: "Update a finding",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(findingUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Patch(context.Background(), "/api/audit/v2/findings/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- plan --------

var auditPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Audit plans",
}

var auditPlanListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit plans",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/audit/v2/plans", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var auditPlanGetCmd = &cobra.Command{
	Use:   "get <plan-id>",
	Short: "Get an audit plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/audit/v2/plans/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	auditWPListCmd.Flags().StringVar(&wpListFile, "file", "", "Optional JSON filter body")
	auditWPCreateCmd.Flags().StringVar(&wpCreateFile, "file", "", "Path to JSON body")
	auditFindingCreateCmd.Flags().StringVar(&findingCreateFile, "file", "", "Path to JSON body")
	auditFindingUpdateCmd.Flags().StringVar(&findingUpdateFile, "file", "", "Path to JSON body")

	auditWPCmd.AddCommand(auditWPListCmd, auditWPGetCmd, auditWPCreateCmd)
	auditFindingCmd.AddCommand(auditFindingListCmd, auditFindingGetCmd, auditFindingCreateCmd, auditFindingUpdateCmd)
	auditPlanCmd.AddCommand(auditPlanListCmd, auditPlanGetCmd)
	auditCmd.AddCommand(auditWPCmd, auditFindingCmd, auditPlanCmd)
	rootCmd.AddCommand(auditCmd)
}
