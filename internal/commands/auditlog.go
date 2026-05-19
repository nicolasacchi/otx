package commands

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
)

var auditlogCmd = &cobra.Command{
	Use:   "auditlog",
	Short: "Login history + activity / change logs",
}

var (
	auditUser   string
	auditAction string
)

var auditLoginCmd = &cobra.Command{
	Use:   "login-history",
	Short: "Login history (GET /api/access/v1/login-history)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("userId", auditUser, "from", fromFlag, "to", toFlag)
		body, err := c.Get(context.Background(), "/api/access/v1/login-history", params)
		if err != nil {
			return err
		}
		return printData("auditlog.login-history", flattenItems(body))
	},
}

var auditActivityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Entity change/activity log",
}

var auditActivitySearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search the activity log",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body := map[string]any{}
		if auditUser != "" {
			body["userId"] = auditUser
		}
		if auditAction != "" {
			body["action"] = auditAction
		}
		if fromFlag != "" {
			body["from"] = fromFlag
		}
		if toFlag != "" {
			body["to"] = toFlag
		}
		resp, err := c.Post(context.Background(), "/api/audit/v1/activity/search", body)
		if err != nil {
			return err
		}
		return printData("auditlog.activity.search", flattenItems(json.RawMessage(resp)))
	},
}

func init() {
	auditLoginCmd.Flags().StringVar(&auditUser, "user", "", "Filter by user ID")
	auditActivitySearchCmd.Flags().StringVar(&auditUser, "user", "", "Filter by user ID")
	auditActivitySearchCmd.Flags().StringVar(&auditAction, "action", "", "Filter by action verb (e.g. 'modified')")

	auditActivityCmd.AddCommand(auditActivitySearchCmd)
	auditlogCmd.AddCommand(auditLoginCmd, auditActivityCmd)
	rootCmd.AddCommand(auditlogCmd)
}
