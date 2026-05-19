package commands

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/nicolasacchi/otx/internal/async"
	"github.com/spf13/cobra"
)

var discoveryCmd = &cobra.Command{
	Use:   "discovery",
	Short: "Data discovery + classification (custom scan jobs against connected data sources)",
}

// -------- scan --------

var discScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Custom scan jobs",
}

var (
	discScanFile     string
	discScanInterval time.Duration
	discScanTimeout  time.Duration
)

var discScanCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a custom scan job",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(discScanFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/data-discovery/v1/jobs", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var discScanListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scan jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/data-discovery/v1/jobs", nil)
		if err != nil {
			return err
		}
		return printData("discovery.scan.list", flattenItems(body))
	},
}

var discScanStatusCmd = &cobra.Command{
	Use:   "status <job-id>",
	Short: "Get a scan job's status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/data-discovery/v1/jobs/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

var discScanResultsCmd = &cobra.Command{
	Use:   "results <job-id>",
	Short: "Get scan job results",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/data-discovery/v1/jobs/"+url.PathEscape(args[0])+"/results", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- classify --------

var discClassifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Submit content for classification",
}

var discClassifyFile string

var discClassifySubmitCmd = &cobra.Command{
	Use:   "submit <job-id>",
	Short: "Submit data for classification within a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(discClassifyFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/data-discovery/v1/jobs/"+url.PathEscape(args[0])+"/classify-data", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var discClassifyResultsCmd = &cobra.Command{
	Use:   "results <job-id>",
	Short: "Get classification results for a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/data-discovery/v1/jobs/"+url.PathEscape(args[0])+"/classify-data/results", nil)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

// -------- detector --------

var discDetectorCmd = &cobra.Command{
	Use:   "detector",
	Short: "Custom classification detectors",
}

var discDetectorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detectors",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), "/api/data-discovery/v1/detectors", nil)
		if err != nil {
			return err
		}
		return printData("discovery.detector.list", flattenItems(body))
	},
}

var discDetectorFile string

var discDetectorCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a custom detector",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		body, err := readJSONFile(discDetectorFile)
		if err != nil {
			return err
		}
		resp, err := c.Post(context.Background(), "/api/data-discovery/v1/detectors", body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

// -------- poll --------

var discPollCmd = &cobra.Command{
	Use:   "poll <job-id>",
	Short: "Wait for a discovery scan job to reach a terminal state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := async.Wait(context.Background(), c,
			"/api/data-discovery/v1/jobs/"+url.PathEscape(args[0]),
			async.Options{Interval: discScanInterval, Timeout: discScanTimeout})
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(body))
	},
}

func init() {
	discScanCreateCmd.Flags().StringVar(&discScanFile, "file", "", "Path to JSON scan-job body")
	discClassifySubmitCmd.Flags().StringVar(&discClassifyFile, "file", "", "Path to JSON classify-data body")
	discDetectorCreateCmd.Flags().StringVar(&discDetectorFile, "file", "", "Path to JSON detector body")
	discPollCmd.Flags().DurationVar(&discScanInterval, "interval", 10*time.Second, "Poll cadence")
	discPollCmd.Flags().DurationVar(&discScanTimeout, "timeout", 60*time.Minute, "Hard timeout")

	discScanCmd.AddCommand(discScanCreateCmd, discScanListCmd, discScanStatusCmd, discScanResultsCmd)
	discClassifyCmd.AddCommand(discClassifySubmitCmd, discClassifyResultsCmd)
	discDetectorCmd.AddCommand(discDetectorListCmd, discDetectorCreateCmd)
	discoveryCmd.AddCommand(discScanCmd, discClassifyCmd, discDetectorCmd, discPollCmd)
	rootCmd.AddCommand(discoveryCmd)
}
