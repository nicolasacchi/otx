package commands

import (
	"context"
	"encoding/json"
	"net/url"
	"os"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
)

var consentCmd = &cobra.Command{
	Use:   "consent",
	Short: "Cookie consent / CMP — receipts, purposes, collection points, data subjects",
}

// -------- receipt --------

var consentReceiptCmd = &cobra.Command{
	Use:   "receipt",
	Short: "Consent receipts (the audit trail of every consent capture)",
}

var (
	receiptListQuery string
	receiptListID    string
)

var consentReceiptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List consent receipts (POST /api/consent/v2/receipts)",
	RunE: runE(func() error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		limit, _ := resolvedLimit()
		body := map[string]any{
			"top": limit,
		}
		if fromFlag != "" {
			body["from"] = fromFlag
		}
		if toFlag != "" {
			body["to"] = toFlag
		}
		if receiptListQuery != "" {
			body["query"] = receiptListQuery
		}
		if receiptListID != "" {
			body["dataSubjectId"] = receiptListID
		}
		resp, err := c.Post(context.Background(), "/api/consent/v2/receipts", body)
		if err != nil {
			return err
		}
		return printData("consent.receipt.list", flattenItems(resp))
	}),
}

var consentReceiptGetCmd = &cobra.Command{
	Use:   "get <data-subject-id>",
	Short: "Fetch every receipt for a single data subject",
	Args:  cobra.ExactArgs(1),
	// RunE wired in init().
}

var receiptCreateFile string

var consentReceiptCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a consent receipt (POST /request/v1/consentreceipts/identified)",
	RunE: runE(func() error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(receiptCreateFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/request/v1/consentreceipts/identified", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	}),
}

var receiptBulkFile string

var consentReceiptBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Bulk-import consent receipts (POST /request/v1/consentreceipts/bulk)",
	RunE: runE(func() error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(receiptBulkFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/request/v1/consentreceipts/bulk", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	}),
}

// -------- group --------

var consentGroupCmd = &cobra.Command{
	Use:   "group",
	Short: "Consent groups / categories",
}

var consentGroupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List consent groups",
	RunE: runE(func() error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v1/consentgroups", nil)
		if err != nil {
			return err
		}
		return printData("consent.group.list", flattenItems(body))
	}),
}

var consentGroupGetCmd = &cobra.Command{
	Use:   "get <group-id>",
	Short: "Get a consent group by ID",
	Args:  cobra.ExactArgs(1),
	// RunE wired in init() so it can access args.
}

var consentGroupSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Get consent group priority settings",
	RunE: runE(func() error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consent/v2/consent-groups/settings", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	}),
}

// -------- purpose --------

var consentPurposeCmd = &cobra.Command{
	Use:   "purpose",
	Short: "Consent purposes",
}

var consentPurposeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List purposes",
	RunE: runE(func() error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v1/purposes", nil)
		if err != nil {
			return err
		}
		return printData("consent.purpose.list", flattenItems(body))
	}),
}

var consentPurposeGetCmd = &cobra.Command{
	Use:   "get <purpose-id>",
	Short: "Get a purpose by ID",
	Args:  cobra.ExactArgs(1),
	// RunE wired in init().
}

// -------- collection-point --------

var consentCPCmd = &cobra.Command{
	Use:   "collection-point",
	Short: "Collection points (web forms, mobile, API endpoints capturing consent)",
}

var consentCPListCmd = &cobra.Command{
	Use:   "list",
	Short: "List collection points",
	RunE: runE(func() error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v1/collectionpoints", nil)
		if err != nil {
			return err
		}
		return printData("consent.collection-point.list", flattenItems(body))
	}),
}

// -------- subject --------

var consentSubjectCmd = &cobra.Command{
	Use:   "subject",
	Short: "Data subjects within the consent platform",
}

var consentSubjectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List data subjects (paginated)",
	RunE: runE(func() error {
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
		return printData("consent.subject.list", body)
	}),
}

var consentSubjectGetCmd = &cobra.Command{
	Use:   "get <data-subject-id>",
	Short: "Get a data subject (v4)",
	Args:  cobra.ExactArgs(1),
	// RunE wired in init().
}

// -------- attachment --------

var consentAttachCmd = &cobra.Command{
	Use:   "attachment",
	Short: "Consent evidence attachments (PDF/JPEG/PNG ≤ 4MB)",
}

func init() {
	consentReceiptListCmd.Flags().StringVar(&receiptListQuery, "query", "", "Optional search query")
	consentReceiptListCmd.Flags().StringVar(&receiptListID, "data-subject-id", "", "Filter to a single data subject")
	consentReceiptCreateCmd.Flags().StringVar(&receiptCreateFile, "file", "", "Path to JSON request body (or '-' for stdin)")
	consentReceiptBulkCmd.Flags().StringVar(&receiptBulkFile, "file", "", "Path to JSON request body (or '-' for stdin)")

	// Wire the get-by-id handlers properly (the runE wrapper above doesn't get
	// access to args; we override Run on these specific commands).
	consentReceiptGetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v1/datasubjects/"+url.PathEscape(args[0])+"/receipts", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	}
	consentGroupGetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consent/v2/consent-groups/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	}
	consentPurposeGetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v1/purposes/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	}
	consentSubjectGetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/consentmanager/v4/datasubjects/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	}

	consentReceiptCmd.AddCommand(consentReceiptListCmd, consentReceiptGetCmd, consentReceiptCreateCmd, consentReceiptBulkCmd)
	consentGroupCmd.AddCommand(consentGroupListCmd, consentGroupGetCmd, consentGroupSettingsCmd)
	consentPurposeCmd.AddCommand(consentPurposeListCmd, consentPurposeGetCmd)
	consentCPCmd.AddCommand(consentCPListCmd)
	consentSubjectCmd.AddCommand(consentSubjectListCmd, consentSubjectGetCmd)
	consentCmd.AddCommand(consentReceiptCmd, consentGroupCmd, consentPurposeCmd, consentCPCmd, consentSubjectCmd, consentAttachCmd)
	rootCmd.AddCommand(consentCmd)
}

// readJSONFile reads a JSON request body from disk or stdin ("-").
func readJSONFile(path string) (any, error) {
	if path == "" {
		return nil, &client.APIError{Kind: "validation", Detail: "--file is required (path or '-' for stdin)"}
	}
	var raw []byte
	var err error
	if path == "-" {
		raw, err = readAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, &client.APIError{Kind: "validation", Detail: "invalid JSON: " + err.Error()}
	}
	return body, nil
}

// getPathJSON is a thin helper for GET-then-print-JSON commands without table
// definitions (kept for one-liners).
func getPathJSON(path string) error {
	c, _, err := getClient()
	if err != nil {
		return err
	}
	body, err := c.Get(context.Background(), path, nil)
	if err != nil {
		return err
	}
	return printJSONValue(json.RawMessage(body))
}

func paginateOpts(limit int) clientPageOpts {
	return clientPageOpts{Size: 50, MaxItems: limit}
}

// clientPageOpts mirrors client.PageOptions but kept here as a lightweight
// alias so command files don't need a direct dependency on the client package
// for option construction.
type clientPageOpts = client.PageOptions

func readAll(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}
