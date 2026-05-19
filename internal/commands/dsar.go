package commands

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/nicolasacchi/otx/internal/async"
	"github.com/spf13/cobra"
)

var dsarCmd = &cobra.Command{
	Use:   "dsar",
	Short: "Data Subject Access Requests (right-to-access / right-to-erasure)",
}

// -------- request --------

var dsarReqCmd = &cobra.Command{
	Use:   "request",
	Short: "DSAR request lifecycle",
}

var (
	dsarListStatus   string
	dsarListType     string
	dsarListAll      bool
	dsarCreateFile   string
	dsarCancelReason string
	dsarStageValue   string
	dsarFieldsFile   string
)

var dsarReqListCmd = &cobra.Command{
	Use:   "list",
	Short: "List DSAR requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("status", dsarListStatus, "requestType", dsarListType)
		limit, _ := resolvedLimit()
		opts := clientPageOpts{Size: 50, MaxItems: limit}
		if dsarListAll {
			opts.MaxItems = 0
		}
		items, err := c.PaginatePage(context.Background(), "/api/dsar/requests", params, opts)
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("dsar.request.list", body)
	},
}

var dsarReqGetCmd = &cobra.Command{
	Use:   "get <request-id>",
	Short: "Get a DSAR request by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/dsar/requests/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var dsarReqCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a DSAR request (POST /api/dsar/requests)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dsarCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/dsar/requests", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dsarReqCancelCmd = &cobra.Command{
	Use:   "cancel <request-id>",
	Short: "Cancel a DSAR request (advances stage to CANCELLED)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body := map[string]any{"stage": "CANCELLED"}
		if dsarCancelReason != "" {
			body["reason"] = dsarCancelReason
		}
		resp, err := c.Put(context.Background(), "/api/dsar/requests/"+url.PathEscape(args[0])+"/stage", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dsarReqFieldsCmd = &cobra.Command{
	Use:   "custom-fields <request-id>",
	Short: "Update custom fields on a request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dsarFieldsFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/dsar/requests/"+url.PathEscape(args[0])+"/custom-fields", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- stage --------

var dsarStageCmd = &cobra.Command{
	Use:   "stage",
	Short: "Advance / update DSAR request stages",
}

var dsarStageUpdateCmd = &cobra.Command{
	Use:   "update <request-id>",
	Short: "Update the request stage (NEW/VERIFYING_IDENTITY/IN_PROGRESS/REJECTED/COMPLETE)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if dsarStageValue == "" {
			return validationErr("--stage is required (NEW/VERIFYING_IDENTITY/IN_PROGRESS/REJECTED/COMPLETE/CANCELLED)")
		}
		body := map[string]any{"stage": dsarStageValue}
		resp, err := c.Put(context.Background(), "/api/dsar/requests/"+url.PathEscape(args[0])+"/stage", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- subtask --------

var dsarSubtaskCmd = &cobra.Command{
	Use:   "subtask",
	Short: "DSAR subtasks (per-system fulfilment steps)",
}

var dsarSubtaskListCmd = &cobra.Command{
	Use:   "list <request-id>",
	Short: "List subtasks for a request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/dsar/requests/"+url.PathEscape(args[0])+"/subtasks", nil)
		if err != nil {
			return err
		}
		return printData("dsar.subtask.list", flattenItems(body))
	},
}

var dsarSubtaskCompleteCmd = &cobra.Command{
	Use:   "complete <request-id> <subtask-id>",
	Short: "Mark a subtask as completed",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Patch(context.Background(),
			"/api/dsar/requests/"+url.PathEscape(args[0])+"/subtasks/"+url.PathEscape(args[1]),
			map[string]any{"status": "COMPLETED"})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- poll --------

var (
	dsarPollInterval time.Duration
	dsarPollTimeout  time.Duration
)

var dsarPollCmd = &cobra.Command{
	Use:   "poll <request-id>",
	Short: "Wait for a DSAR request to reach a terminal stage",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := async.Wait(context.Background(), c,
			"/api/dsar/requests/"+url.PathEscape(args[0]),
			async.Options{
				Interval:       dsarPollInterval,
				Timeout:        dsarPollTimeout,
				StatusField:    "stage",
				TerminalStates: []string{"COMPLETE", "COMPLETED", "REJECTED", "CANCELLED", "FAILED"},
			})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	dsarReqListCmd.Flags().StringVar(&dsarListStatus, "status", "", "Filter by stage")
	dsarReqListCmd.Flags().StringVar(&dsarListType, "type", "", "Filter by request type")
	dsarReqListCmd.Flags().BoolVar(&dsarListAll, "all", false, "Fetch all pages")
	dsarReqCreateCmd.Flags().StringVar(&dsarCreateFile, "file", "", "Path to JSON request body (or '-' for stdin)")
	dsarReqCancelCmd.Flags().StringVar(&dsarCancelReason, "reason", "", "Cancellation reason")
	dsarReqFieldsCmd.Flags().StringVar(&dsarFieldsFile, "file", "", "Path to JSON custom-fields body (or '-' for stdin)")
	dsarStageUpdateCmd.Flags().StringVar(&dsarStageValue, "stage", "", "Target stage")
	dsarPollCmd.Flags().DurationVar(&dsarPollInterval, "interval", 5*time.Second, "Poll cadence")
	dsarPollCmd.Flags().DurationVar(&dsarPollTimeout, "timeout", 10*time.Minute, "Hard timeout")

	dsarReqCmd.AddCommand(dsarReqListCmd, dsarReqGetCmd, dsarReqCreateCmd, dsarReqCancelCmd, dsarReqFieldsCmd)
	dsarStageCmd.AddCommand(dsarStageUpdateCmd)
	dsarSubtaskCmd.AddCommand(dsarSubtaskListCmd, dsarSubtaskCompleteCmd)
	dsarCmd.AddCommand(dsarReqCmd, dsarStageCmd, dsarSubtaskCmd, dsarPollCmd)
	rootCmd.AddCommand(dsarCmd)
}
