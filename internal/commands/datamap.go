package commands

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

var datamapCmd = &cobra.Command{
	Use:   "datamap",
	Short: "Data mapping / RoPA / inventory (system, data element, processing activity, asset)",
}

// -------- inventory --------

var dmInvCmd = &cobra.Command{
	Use:   "inventory",
	Short: "Inventory CRUD across all types (--type)",
}

var (
	invType        string
	invCreateFile  string
	invUpdateFile  string
	invExternalRef string
)

var dmInvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List inventory items of a given type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required (vendor / system / dataelement / processingactivity / risk / finding / asset)")
		}
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		items, err := c.PaginatePage(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType), nil, clientPageOpts{Size: 50, MaxItems: limit})
		if err != nil {
			return err
		}
		body, _ := json.Marshal(items)
		return printData("datamap.inventory.list", body)
	},
}

var dmInvGetCmd = &cobra.Command{
	Use:   "get <inventory-id>",
	Short: "Get an inventory item",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var dmInvCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an inventory item (POST)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(invCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dmInvUpdateCmd = &cobra.Command{
	Use:   "update <inventory-id>",
	Short: "Update an inventory item (PUT)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(invUpdateFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dmInvDeleteCmd = &cobra.Command{
	Use:   "delete <inventory-id>",
	Short: "Delete an inventory item",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/"+url.PathEscape(args[0])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true, "id": args[0]})
	},
}

var dmInvUpsertCmd = &cobra.Command{
	Use:   "upsert-by-ref <external-id>",
	Short: "Create-or-update by external reference (PUT /reference/{externalId})",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(invCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/reference/"+url.PathEscape(args[0]), body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- link --------

var dmLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Inventory relations (link/unlink)",
}

var dmLinkListCmd = &cobra.Command{
	Use:   "list <inventory-id>",
	Short: "List relations for an inventory item",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/"+url.PathEscape(args[0])+"/links", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var dmLinkAddFile string

var dmLinkAddCmd = &cobra.Command{
	Use:   "add <inventory-id>",
	Short: "Add a relation to an inventory item",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dmLinkAddFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/"+url.PathEscape(args[0])+"/links", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dmLinkRemoveCmd = &cobra.Command{
	Use:   "remove <inventory-id> <link-id>",
	Short: "Remove a relation",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if err := c.Delete(context.Background(), "/api/inventory/v2/inventories/"+url.PathEscape(invType)+"/"+url.PathEscape(args[0])+"/links/"+url.PathEscape(args[1])); err != nil {
			return err
		}
		return printJSONValue(map[string]any{"deleted": true})
	},
}

// -------- ropa --------

var dmRopaCmd = &cobra.Command{
	Use:   "ropa",
	Short: "Record of Processing Activities (RoPA)",
}

var dmRopaGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Trigger a RoPA regeneration",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/ropa/generate", map[string]any{})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dmRopaExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current RoPA",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/ropa/export", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- classification + category --------

var dmClassCmd = &cobra.Command{
	Use:   "classification",
	Short: "Data classifications",
}

var dmClassFile string

var dmClassListCmd = &cobra.Command{
	Use:   "list",
	Short: "List data classifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/data-classifications", nil)
		if err != nil {
			return err
		}
		return printData("datamap.classification.list", flattenItems(body))
	},
}

var dmClassCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a data classification",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dmClassFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/data-classifications", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dmCatCmd = &cobra.Command{
	Use:   "category",
	Short: "Data categories",
}

var dmCatCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a data category",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dmClassFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/data-categories", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- schema --------

var dmSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Inventory schemas + custom attribute management",
}

var dmSchemaGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get schema for an inventory type (--type)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/schemas/"+url.PathEscape(invType), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var dmSchemaAttrListCmd = &cobra.Command{
	Use:   "attributes-list",
	Short: "List schema attributes for --type",
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/inventory/v2/schemas/"+url.PathEscape(invType)+"/attributes", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var dmSchemaAttrAddFile string

var dmSchemaAttrAddCmd = &cobra.Command{
	Use:   "attribute-add",
	Short: "Add a custom attribute to a schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dmSchemaAttrAddFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/inventory/v2/schemas/"+url.PathEscape(invType)+"/attributes", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var dmSchemaAttrActFile string

var dmSchemaAttrActivateCmd = &cobra.Command{
	Use:   "attribute-activate",
	Short: "Activate attributes by field name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if invType == "" {
			return validationErr("--type required")
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(dmSchemaAttrActFile)
		if err != nil {
			return err
		}
		resp, err := c.Put(context.Background(), "/api/inventory/v2/schemas/"+url.PathEscape(invType)+"/attributes/activate", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

func init() {
	dmInvCmd.PersistentFlags().StringVar(&invType, "type", "", "Inventory type: vendor / system / dataelement / processingactivity / risk / finding / asset")
	dmLinkCmd.PersistentFlags().StringVar(&invType, "type", "", "Inventory type")
	dmSchemaCmd.PersistentFlags().StringVar(&invType, "type", "", "Inventory type")
	dmInvCreateCmd.Flags().StringVar(&invCreateFile, "file", "", "Path to JSON body")
	dmInvUpdateCmd.Flags().StringVar(&invUpdateFile, "file", "", "Path to JSON body")
	dmInvUpsertCmd.Flags().StringVar(&invCreateFile, "file", "", "Path to JSON body")
	dmLinkAddCmd.Flags().StringVar(&dmLinkAddFile, "file", "", "Path to JSON body")
	dmClassCreateCmd.Flags().StringVar(&dmClassFile, "file", "", "Path to JSON body")
	dmCatCreateCmd.Flags().StringVar(&dmClassFile, "file", "", "Path to JSON body")
	dmSchemaAttrAddCmd.Flags().StringVar(&dmSchemaAttrAddFile, "file", "", "Path to JSON body")
	dmSchemaAttrActivateCmd.Flags().StringVar(&dmSchemaAttrActFile, "file", "", "Path to JSON body")

	dmInvCmd.AddCommand(dmInvListCmd, dmInvGetCmd, dmInvCreateCmd, dmInvUpdateCmd, dmInvDeleteCmd, dmInvUpsertCmd)
	dmLinkCmd.AddCommand(dmLinkListCmd, dmLinkAddCmd, dmLinkRemoveCmd)
	dmRopaCmd.AddCommand(dmRopaGenerateCmd, dmRopaExportCmd)
	dmClassCmd.AddCommand(dmClassListCmd, dmClassCreateCmd)
	dmCatCmd.AddCommand(dmCatCreateCmd)
	dmSchemaCmd.AddCommand(dmSchemaGetCmd, dmSchemaAttrListCmd, dmSchemaAttrAddCmd, dmSchemaAttrActivateCmd)
	datamapCmd.AddCommand(dmInvCmd, dmLinkCmd, dmRopaCmd, dmClassCmd, dmCatCmd, dmSchemaCmd)
	rootCmd.AddCommand(datamapCmd)
}
