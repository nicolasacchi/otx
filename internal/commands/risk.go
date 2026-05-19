package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var riskCmd = &cobra.Command{
	Use:   "risk",
	Short: "IT risk management (lifecycle: identification → mitigation)",
}

// -------- it-risk --------

var itRiskCmd = &cobra.Command{
	Use:   "it-risk",
	Short: "IT risks",
}

var (
	itRiskCreateFile string
	itRiskUpdateFile string
)

var itRiskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List IT risks",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/risk/v2/risks", nil, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("risk.list", body)
	},
}

var itRiskGetCmd = &cobra.Command{
	Use:   "get <risk-id>",
	Short: "Get a risk by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/risk/v2/risks/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var itRiskCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a risk",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(itRiskCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/risk/v2/risks", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var itRiskUpdateCmd = &cobra.Command{
	Use:   "update <risk-id>",
	Short: "Update a risk",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(itRiskUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/risk/v2/risks/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	itRiskCreateCmd.Flags().StringVar(&itRiskCreateFile, "file", "", "Path to JSON body")
	itRiskUpdateCmd.Flags().StringVar(&itRiskUpdateFile, "file", "", "Path to JSON body")
	itRiskCmd.AddCommand(itRiskListCmd, itRiskGetCmd, itRiskCreateCmd, itRiskUpdateCmd)
	riskCmd.AddCommand(itRiskCmd)
	rootCmd.AddCommand(riskCmd)
}
