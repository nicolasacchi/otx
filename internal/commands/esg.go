package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var esgCmd = &cobra.Command{
	Use:   "esg",
	Short: "ESG metrics/frameworks/reports (key /api/esg-management/v1/metrics endpoints deprecated June 2025)",
}

// warnDeprecated prints the standard sunset banner. Endpoint still attempted
// after warning so users can capture residual responses.
func warnDeprecated(endpoint string) {
	fmt.Fprintf(os.Stderr,
		"⚠  %s: this OneTrust endpoint was deprecated 2025-06-08; consult OneTrust's API sunsetting guidelines for replacements.\n",
		endpoint)
}

// -------- metric --------

var esgMetricCmd = &cobra.Command{
	Use:   "metric",
	Short: "ESG metrics (DEPRECATED June 2025)",
}

var (
	esgMetricFile string
	esgMetricFrom string
	esgMetricTo   string
)

var esgMetricListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ESG metrics for a period",
	RunE: func(cmd *cobra.Command, args []string) error {
		warnDeprecated("/api/esg-management/v1/metrics/metric-details")
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body := map[string]any{}
		if esgMetricFrom != "" {
			body["from"] = esgMetricFrom
		}
		if esgMetricTo != "" {
			body["to"] = esgMetricTo
		}
		if esgMetricFile != "" {
			parsed, err := readJSONFile(esgMetricFile)
			if err != nil {
				return err
			}
			body = parsed.(map[string]any)
		}
		resp, err := c.Post(context.Background(), "/api/esg-management/v1/metrics/metric-details", body)
		if err != nil {
			return err
		}
		return printData("esg.metric.list", flattenItems(json.RawMessage(resp)))
	},
}

var esgMetricGetCmd = &cobra.Command{
	Use:   "get <metric-id>",
	Short: "Get details for a specific ESG metric",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		warnDeprecated("/api/esg-management/v1/metrics/{id}")
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/esg-management/v1/metrics/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- framework --------

var esgFrameworkCmd = &cobra.Command{
	Use:   "framework",
	Short: "ESG reporting frameworks (SASB / TCFD / GRI / ISSB)",
}

var esgFrameworkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported frameworks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/esg-management/v1/frameworks", nil)
		if err != nil {
			return err
		}
		return printData("esg.framework.list", flattenItems(body))
	},
}

// -------- report --------

var esgReportCmd = &cobra.Command{
	Use:   "report",
	Short: "ESG reporting + export",
}

var esgReportFile string

var esgReportGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Trigger an ESG report generation job",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(esgReportFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/esg-management/v1/reports/generate", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var esgReportExportCmd = &cobra.Command{
	Use:   "export <report-id>",
	Short: "Export a generated ESG report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/esg-management/v1/reports/"+url.PathEscape(args[0])+"/export", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	esgMetricListCmd.Flags().StringVar(&esgMetricFile, "file", "", "Optional JSON filter body")
	esgMetricListCmd.Flags().StringVar(&esgMetricFrom, "from", "", "Period start")
	esgMetricListCmd.Flags().StringVar(&esgMetricTo, "to", "", "Period end")
	esgReportGenerateCmd.Flags().StringVar(&esgReportFile, "file", "", "Path to JSON body")

	esgMetricCmd.AddCommand(esgMetricListCmd, esgMetricGetCmd)
	esgFrameworkCmd.AddCommand(esgFrameworkListCmd)
	esgReportCmd.AddCommand(esgReportGenerateCmd, esgReportExportCmd)
	esgCmd.AddCommand(esgMetricCmd, esgFrameworkCmd, esgReportCmd)
	rootCmd.AddCommand(esgCmd)
}
