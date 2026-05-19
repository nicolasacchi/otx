package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Event subscriptions (webhooks for assessment/incident/DSAR/etc lifecycle events)",
}

var webhookCreateFile string

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhook subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/webhook/v1/subscriptions", nil)
		if err != nil {
			return err
		}
		return printData("webhook.list", flattenItems(body))
	},
}

var webhookGetCmd = &cobra.Command{
	Use:   "get <subscription-id>",
	Short: "Get a webhook subscription by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/webhook/v1/subscriptions/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var webhookSubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to events (POST /api/webhook/v1/subscriptions)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(webhookCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/webhook/v1/subscriptions", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var webhookDeleteCmd = &cobra.Command{
	Use:   "delete <subscription-id>",
	Short: "Delete a webhook subscription",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/webhook/v1/subscriptions/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

// -------- event --------

var webhookEventCmd = &cobra.Command{
	Use:   "event",
	Short: "Webhook event history + replay",
}

var webhookEventListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent webhook events",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/webhook/v1/events", nil)
		if err != nil {
			return err
		}
		return printData("webhook.event.list", flattenItems(body))
	},
}

var webhookEventReplayCmd = &cobra.Command{
	Use:   "replay <event-id>",
	Short: "Replay a webhook event",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(),
			"/api/webhook/v1/events/"+url.PathEscape(args[0])+"/replay", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	webhookSubscribeCmd.Flags().StringVar(&webhookCreateFile, "file", "", "Path to JSON subscription body (or '-')")
	webhookEventCmd.AddCommand(webhookEventListCmd, webhookEventReplayCmd)
	webhookCmd.AddCommand(webhookListCmd, webhookGetCmd, webhookSubscribeCmd, webhookDeleteCmd, webhookEventCmd)
	rootCmd.AddCommand(webhookCmd)
}
