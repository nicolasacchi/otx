package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/nicolasacchi/otx/internal/config"
	"github.com/nicolasacchi/otx/internal/output"
	"github.com/spf13/cobra"
)

var (
	version          = "dev"
	clientIDFlag     string
	clientSecretFlag string
	apiKeyFlag       string
	baseURLFlag      string
	projectFlag      string
	jsonFlag         bool
	jqFlag           string
	verboseFlag      bool
	quietFlag        bool
	fromFlag         string
	toFlag           string
	limitFlag        int
	rowsFlag         int
	confirmFlag      bool
	timingFlag       bool
)

// AgentCap is the row limit applied under CLAUDECODE=1.
const AgentCap = 100

var rootCmd = &cobra.Command{
	Use:   "otx",
	Short: "otx — OneTrust Explorer CLI",
	Long: `otx is a Go CLI for the OneTrust REST API. Covers cookie consent (CMP),
universal consent & preference management (UCPM), DSAR, data mapping,
third-party risk, GRC, SCIM users, and platform-wide services.

Usage examples:
  otx consent receipt list --from 24h
  otx consent purpose list
  otx ucpm preference get <data-subject-id>
  otx auth token get
  otx config add production-eu --client-id ID --client-secret SECRET --base-url https://app-eu.onetrust.com`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// SetVersion injects the build-time version string.
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// Execute runs the root command and returns any error for main.go to dispatch.
func Execute() error {
	return rootCmd.Execute()
}

// getClient builds a read-mode client (no --confirm needed).
func getClient() (*client.Client, *config.Credentials, error) {
	return loadClient(false)
}

// getWriteClient builds a write-mode client. Requires --confirm AND write
// credentials (either via flags, OTX_WRITE_CLIENT_* env, or write_client_id in
// config). Returns a write_locked APIError when --confirm is missing.
func getWriteClient() (*client.Client, *config.Credentials, error) {
	if !confirmFlag {
		return nil, nil, &client.APIError{
			Kind:   "write_locked",
			Detail: "write operation requires --confirm flag",
			Hint:   "re-run with --confirm and ensure OTX_WRITE_CLIENT_ID/SECRET (or write_client_id in config) are set",
		}
	}
	return loadClient(true)
}

func loadClient(writeMode bool) (*client.Client, *config.Credentials, error) {
	creds, err := config.LoadCredentials(config.LoadOptions{
		ClientIDFlag:     clientIDFlag,
		ClientSecretFlag: clientSecretFlag,
		APIKeyFlag:       apiKeyFlag,
		BaseURLFlag:      baseURLFlag,
		ProjectFlag:      projectFlag,
		WriteMode:        writeMode,
	})
	if err != nil {
		return nil, nil, &client.APIError{Kind: "auth_failed", Detail: err.Error(), Hint: "run 'otx config doctor' to verify your setup"}
	}
	// An API key authenticates directly as a static bearer; otherwise exchange
	// OAuth client credentials.
	var tokens *client.TokenProvider
	if creds.APIKey != "" {
		tokens = client.NewStaticTokenProvider(creds.BaseURL, creds.APIKey, verboseFlag)
	} else {
		tokens = client.NewTokenProvider(creds.BaseURL, creds.ClientID, creds.ClientSecret, verboseFlag)
	}
	return client.New(creds.BaseURL, tokens, verboseFlag), creds, nil
}

// resolvedLimit returns the effective --limit, honoring CLAUDECODE caps unless
// the user passed --rows to override.
func resolvedLimit() (int, bool) {
	override := rowsFlag
	if override == 0 && !rootCmd.PersistentFlags().Changed("rows") {
		override = -1
	}
	return output.ResolveLimit(limitFlag, AgentCap, override)
}

func isJSONMode() bool {
	return output.IsJSON(jsonFlag, jqFlag)
}

func printData(command string, data []byte) error {
	_, truncated := resolvedLimit()
	return output.PrintData(command, data, isJSONMode(), jqFlag, truncated)
}

// printJSONValue prints any Go value through the standard JSON pipeline.
func printJSONValue(v any) error {
	return output.PrintJSONValue(v)
}

// ctx returns a request context. Cobra commands should pass this to the client
// so that signals propagate.
func ctx() context.Context { return context.Background() }

// runE wraps a command runner so panics from missing creds surface cleanly.
func runE(fn func() error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := fn()
		if err == nil {
			return nil
		}
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return apiErr
		}
		return err
	}
}

// formatHint returns a Cobra usage error wrapping a plain text reason. Use
// when validating CLI arguments inside Run handlers.
func validationErr(format string, args ...any) error {
	return &client.APIError{
		Kind:   "validation",
		Detail: fmt.Sprintf(format, args...),
		Hint:   "see 'otx <command> --help' for the expected shape",
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&clientIDFlag, "client-id", "", "OneTrust OAuth client_id (overrides OTX_CLIENT_ID)")
	rootCmd.PersistentFlags().StringVar(&clientSecretFlag, "client-secret", "", "OneTrust OAuth client_secret (overrides OTX_CLIENT_SECRET)")
	rootCmd.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "OneTrust API key used as a direct bearer token (overrides OTX_API_KEY; alternative to --client-id/--client-secret)")
	rootCmd.PersistentFlags().StringVar(&baseURLFlag, "base-url", "", "OneTrust tenant base URL (overrides OTX_BASE_URL; default app-eu.onetrust.com)")
	rootCmd.PersistentFlags().StringVar(&projectFlag, "project", "", "Use a named project from ~/.config/otx/config.toml")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Force JSON output (auto-enabled when stdout is not a TTY)")
	rootCmd.PersistentFlags().StringVar(&jqFlag, "jq", "", "Apply gjson path filter to JSON output (NOT real jq — see docs)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Print request/response details to stderr")
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Suppress non-error output")
	rootCmd.PersistentFlags().StringVar(&fromFlag, "from", "", "Time range start (e.g. 1h, 7d, now-2h, RFC3339)")
	rootCmd.PersistentFlags().StringVar(&toFlag, "to", "", "Time range end (default: now)")
	rootCmd.PersistentFlags().IntVar(&limitFlag, "limit", 50, "Max results to return")
	rootCmd.PersistentFlags().IntVar(&rowsFlag, "rows", -1, "Override CLAUDECODE row cap (0 = unlimited)")
	rootCmd.PersistentFlags().BoolVar(&confirmFlag, "confirm", false, "Required for any write operation (POST/PUT/PATCH/DELETE)")
	rootCmd.PersistentFlags().BoolVar(&timingFlag, "timing", false, "Print per-request timing to stderr")
}
