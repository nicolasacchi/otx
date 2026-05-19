package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policy & compliance management (policies, privacy notices, templates)",
}

// -------- policy --------

var policyPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policies",
}

var (
	policyListType   string
	policyCreateFile string
	policyUpdateFile string
)

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List policies (optionally filter by --compliance-type)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		path := "/api/policy/v2/policies"
		if policyListType != "" {
			path = "/api/enterprise-policy/v1/" + url.PathEscape(policyListType) + "/list"
		}
		body, err := c.Get(context.Background(), path, nil)
		if err != nil {
			return err
		}
		return printData("policy.policy.list", flattenItems(body))
	},
}

var policyGetCmd = &cobra.Command{
	Use:   "get <policy-id>",
	Short: "Get a policy by ID (requires --compliance-type for enterprise-policy endpoints)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		path := "/api/policy/v2/policies/" + url.PathEscape(args[0])
		if policyListType != "" {
			path = "/api/enterprise-policy/v1/" + url.PathEscape(policyListType) + "/" + url.PathEscape(args[0])
		}
		body, err := c.Get(context.Background(), path, nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var policyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(policyCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/policy/v2/policies", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var policyUpdateCmd = &cobra.Command{
	Use:   "update <policy-id>",
	Short: "Update a policy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(policyUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/policy/v2/policies/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- notice --------

var policyNoticeCmd = &cobra.Command{
	Use:   "notice",
	Short: "Privacy notices",
}

var policyNoticeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List privacy notices (paged)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/v2/privacynotices", nil, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("policy.notice.list", body)
	},
}

var policyNoticeGetCmd = &cobra.Command{
	Use:   "get <notice-id>",
	Short: "Get a privacy notice",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/v2/privacynotices/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- template --------

var policyTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Policy templates (50+ compliance frameworks)",
}

var policyTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List policy templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/policy/v2/templates", nil)
		if err != nil {
			return err
		}
		return printData("policy.template.list", flattenItems(body))
	},
}

func init() {
	policyListCmd.Flags().StringVar(&policyListType, "compliance-type", "", "Use /api/enterprise-policy/v1/{type}/ endpoints")
	policyGetCmd.Flags().StringVar(&policyListType, "compliance-type", "", "(same)")
	policyCreateCmd.Flags().StringVar(&policyCreateFile, "file", "", "Path to JSON body")
	policyUpdateCmd.Flags().StringVar(&policyUpdateFile, "file", "", "Path to JSON body")

	policyPolicyCmd.AddCommand(policyListCmd, policyGetCmd, policyCreateCmd, policyUpdateCmd)
	policyNoticeCmd.AddCommand(policyNoticeListCmd, policyNoticeGetCmd)
	policyTemplateCmd.AddCommand(policyTemplateListCmd)
	policyCmd.AddCommand(policyPolicyCmd, policyNoticeCmd, policyTemplateCmd)
	rootCmd.AddCommand(policyCmd)
}
