package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var ucpmCmd = &cobra.Command{
	Use:   "ucpm",
	Short: "Universal Consent & Preference Management (preference center, marketing/email consents)",
}

// -------- preference --------

var ucpmPrefCmd = &cobra.Command{
	Use:   "preference",
	Short: "Data-subject preference get/update",
}

var ucpmPrefGetCmd = &cobra.Command{
	Use:   "get <data-subject-id>",
	Short: "Get preferences for a data subject (v4)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v4/datasubjects/"+url.PathEscape(args[0])+"/preferences", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var ucpmPrefUpdateFile string

var ucpmPrefUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update preferences (POST /api/consent/v2/preferences)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(ucpmPrefUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/consent/v2/preferences", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- subject --------

var ucpmSubjectCmd = &cobra.Command{
	Use:   "subject",
	Short: "UCPM data subjects (v4)",
}

var ucpmSubjectGetCmd = &cobra.Command{
	Use:   "get <data-subject-id>",
	Short: "Get a UCPM data subject by ID (v4)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v4/datasubjects/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var ucpmSubjectCreateFile string

var ucpmSubjectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create / upsert a UCPM data subject (v4)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(ucpmSubjectCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/consentmanager/v4/datasubjects", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var ucpmSubjectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List UCPM data subjects (paginated)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/consentmanager/v2/datasubjects", nil, paginateOpts(limit))
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("ucpm.subject.list", body)
	},
}

// -------- consent-group (UCPM-specific alias) --------

var ucpmGroupCmd = &cobra.Command{
	Use:   "consent-group",
	Short: "UCPM consent groups (same API as 'otx consent group')",
}

var ucpmGroupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List consent groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v1/consentgroups", nil)
		if err != nil {
			return err
		}
		return printData("consent.group.list", flattenItems(body))
	},
}

func init() {
	ucpmPrefUpdateCmd.Flags().StringVar(&ucpmPrefUpdateFile, "file", "", "Path to JSON request body (or '-' for stdin)")
	ucpmSubjectCreateCmd.Flags().StringVar(&ucpmSubjectCreateFile, "file", "", "Path to JSON request body (or '-' for stdin)")

	ucpmPrefCmd.AddCommand(ucpmPrefGetCmd, ucpmPrefUpdateCmd)
	ucpmSubjectCmd.AddCommand(ucpmSubjectGetCmd, ucpmSubjectCreateCmd, ucpmSubjectListCmd)
	ucpmGroupCmd.AddCommand(ucpmGroupListCmd)
	ucpmCmd.AddCommand(ucpmPrefCmd, ucpmSubjectCmd, ucpmGroupCmd)
	rootCmd.AddCommand(ucpmCmd)
}
