package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/nicolasacchi/otx/internal/async"
	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Bulk exports (CONSENT_RECEIPTS, COOKIE_RECEIPTS, DATA_SUBJECTS)",
}

// -------- bulk --------

var exportBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Bulk export jobs (10-day retention)",
}

var (
	exportBulkType     string
	exportBulkFrom     string
	exportBulkTo       string
	exportBulkAwait    bool
	exportBulkInterval time.Duration
	exportBulkTimeout  time.Duration
	exportBulkOut      string
)

var exportBulkCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a bulk-export job",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if exportBulkType == "" {
			return validationErr("--type is required (CONSENT_RECEIPTS / COOKIE_RECEIPTS / DATA_SUBJECTS)")
		}
		body := map[string]any{"type": exportBulkType}
		if exportBulkFrom != "" {
			body["from"] = exportBulkFrom
		}
		if exportBulkTo != "" {
			body["to"] = exportBulkTo
		}
		resp, err := c.Post(context.Background(), "/api/bulk-export/v1/exports", body)
		if err != nil {
			return err
		}
		jobID := gjson.GetBytes(resp, "id").String()
		if jobID == "" {
			jobID = gjson.GetBytes(resp, "exportId").String()
		}
		if !exportBulkAwait {
			return printJSONValue(json.RawMessage(resp))
		}
		// Poll for completion, optionally download.
		final, err := async.Wait(context.Background(), c,
			"/api/bulk-export/v1/exports/"+url.PathEscape(jobID),
			async.Options{Interval: exportBulkInterval, Timeout: exportBulkTimeout})
		if err != nil {
			return err
		}
		if exportBulkOut != "" {
			dl := gjson.GetBytes(final, "downloadUrl").String()
			if dl == "" {
				return validationErr("export finished but no downloadUrl in response")
			}
			if err := downloadPresigned(c, dl, exportBulkOut); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "→ wrote %s\n", exportBulkOut)
		}
		return printJSONValue(json.RawMessage(final))
	},
}

var exportBulkStatusCmd = &cobra.Command{
	Use:   "status <export-id>",
	Short: "Check status of a bulk-export job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/bulk-export/v1/exports/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var exportBulkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent bulk-export jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/bulk-export/v1/exports", nil)
		if err != nil {
			return err
		}
		return printData("export.bulk.list", flattenItems(body))
	},
}

var exportBulkDownloadCmd = &cobra.Command{
	Use:   "download <export-id>",
	Short: "Download a finished export (CSV/JSON)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/bulk-export/v1/exports/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		dl := gjson.GetBytes(body, "downloadUrl").String()
		if dl == "" {
			return validationErr("export does not have a downloadUrl (state may not be COMPLETED)")
		}
		out := exportBulkOut
		if out == "" {
			out = fmt.Sprintf("export-%s", args[0])
		}
		return downloadPresigned(c, dl, out)
	},
}

// -------- poll --------

var exportPollCmd = &cobra.Command{
	Use:   "poll <export-id>",
	Short: "Block until an export job finishes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := async.Wait(context.Background(), c,
			"/api/bulk-export/v1/exports/"+url.PathEscape(args[0]),
			async.Options{Interval: exportBulkInterval, Timeout: exportBulkTimeout})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func downloadPresigned(c *client.Client, urlStr, out string) error {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return &client.APIError{Status: resp.StatusCode, Kind: "api_error", Detail: "presigned download failed"}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func init() {
	exportBulkCreateCmd.Flags().StringVar(&exportBulkType, "type", "", "Export type: CONSENT_RECEIPTS / COOKIE_RECEIPTS / DATA_SUBJECTS")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkFrom, "from", "", "Start date (RFC3339 or yyyy-mm-dd)")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkTo, "to", "", "End date")
	exportBulkCreateCmd.Flags().BoolVar(&exportBulkAwait, "await", false, "Poll until COMPLETED before returning")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkOut, "out", "", "If set with --await, download the file to this path")
	exportBulkDownloadCmd.Flags().StringVar(&exportBulkOut, "out", "", "Output path (default: export-<id>)")
	exportPollCmd.Flags().DurationVar(&exportBulkInterval, "interval", 5*time.Second, "Poll cadence")
	exportPollCmd.Flags().DurationVar(&exportBulkTimeout, "timeout", 30*time.Minute, "Hard timeout")
	exportBulkCreateCmd.Flags().DurationVar(&exportBulkInterval, "interval", 5*time.Second, "Poll cadence (with --await)")
	exportBulkCreateCmd.Flags().DurationVar(&exportBulkTimeout, "timeout", 30*time.Minute, "Hard timeout (with --await)")

	exportBulkCmd.AddCommand(exportBulkCreateCmd, exportBulkStatusCmd, exportBulkListCmd, exportBulkDownloadCmd)
	exportCmd.AddCommand(exportBulkCmd, exportPollCmd)
	rootCmd.AddCommand(exportCmd)
}
