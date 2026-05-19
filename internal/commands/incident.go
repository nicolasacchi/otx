package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var incidentCmd = &cobra.Command{
	Use:   "incident",
	Short: "Incident management (breach/privacy/security incident workflow)",
}

var (
	incCreateFile  string
	incUpdateFile  string
	incListStage   string
	incAdvanceTo   string
)

var incListCmd = &cobra.Command{
	Use:   "list",
	Short: "List incidents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		params := paramsWith("stage", incListStage)
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/incident/v2/incidents", params, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("incident.list", body)
	},
}

var incGetCmd = &cobra.Command{
	Use:   "get <incident-id>",
	Short: "Get an incident by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/incident/v2/incidents/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var incCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an incident",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(incCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/incident/v2/incidents", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var incUpdateCmd = &cobra.Command{
	Use:   "update <incident-id>",
	Short: "Update an incident (PATCH)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(incUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Patch(context.Background(), "/api/incident/v2/incidents/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var incAdvanceCmd = &cobra.Command{
	Use:   "workflow-advance <incident-id>",
	Short: "Advance the incident workflow to a new stage (--to)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if incAdvanceTo == "" {
			return validationErr("--to is required (target stage)")
		}
		resp, err := c.Patch(context.Background(),
			"/api/incident/v2/incidents/"+url.PathEscape(args[0]),
			map[string]any{"stage": incAdvanceTo})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	incListCmd.Flags().StringVar(&incListStage, "stage", "", "Filter by stage")
	incCreateCmd.Flags().StringVar(&incCreateFile, "file", "", "Path to JSON body")
	incUpdateCmd.Flags().StringVar(&incUpdateFile, "file", "", "Path to JSON patch body")
	incAdvanceCmd.Flags().StringVar(&incAdvanceTo, "to", "", "Target workflow stage")

	incidentCmd.AddCommand(incListCmd, incGetCmd, incCreateCmd, incUpdateCmd, incAdvanceCmd)
	rootCmd.AddCommand(incidentCmd)
}
