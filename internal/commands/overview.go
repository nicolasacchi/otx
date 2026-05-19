package commands

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var overviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Parallel fan-out across modules — single-call OneTrust health snapshot",
	Long: `overview fans out parallel calls across every product surface otx covers and
returns a single rolled-up JSON object. Useful as a "is anything on fire?"
snapshot or a daily-changelog data source.

Sections (each runs concurrently with its own error envelope on failure):
  consent       — receipt count over the last --window
  dsar          — open requests (NEW + VERIFYING_IDENTITY + IN_PROGRESS counts)
  assessment    — under-review + pending counts
  incident      — recently created
  tprm          — vendors with stale assessments
  webhooks      — active subscription count + recent event count
  auth          — current token's expires_in_seconds
  scopes        — count of granted OAuth scopes

Failures in any single section do NOT abort the call — each goroutine
captures its own error and surfaces it in the result map as an error
envelope under the section name (e.g. "dsar_error": {...}).`,
}

var overviewWindow string

func init() {
	overviewCmd.Flags().StringVar(&overviewWindow, "window", "24h", "Time window used for consent/incident/auditlog sections")
	overviewCmd.RunE = runOverview
	rootCmd.AddCommand(overviewCmd)
}

func runOverview(cmd *cobra.Command, args []string) error {
	c, creds, err := getClient()
	if err != nil {
		return err
	}

	result := map[string]any{
		"project":  creds.ProjectName,
		"base_url": creds.BaseURL,
		"window":   overviewWindow,
		"at":       time.Now().UTC().Format(time.RFC3339),
	}
	var mu sync.Mutex
	var wg sync.WaitGroup

	probe := func(key string, fn func() (any, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := fn()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result[key+"_error"] = sanitizeError(err)
				return
			}
			result[key] = v
		}()
	}

	ctx := context.Background()

	probe("consent_receipts", func() (any, error) {
		body, err := c.Post(ctx, "/api/consent/v2/receipts",
			map[string]any{"top": 1, "from": overviewWindow})
		if err != nil {
			return nil, err
		}
		total := gjson.GetBytes(body, "totalRecords").Int()
		if total == 0 {
			total = gjson.GetBytes(body, "totalResults").Int()
		}
		return map[string]any{"total": total, "window": overviewWindow}, nil
	})

	probe("dsar_open", func() (any, error) {
		body, err := c.Get(ctx, "/api/dsar/requests", paramsWith("status", "IN_PROGRESS"))
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"count": gjson.GetBytes(body, "totalElements").Int(),
		}, nil
	})

	probe("assessment_under_review", func() (any, error) {
		body, err := c.Get(ctx, "/api/assessment/v2/assessments", paramsWith("stage", "UNDER_REVIEW"))
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"count": gjson.GetBytes(body, "totalElements").Int(),
		}, nil
	})

	probe("incidents_recent", func() (any, error) {
		body, err := c.Get(ctx, "/api/incident/v2/incidents", paramsWith("from", overviewWindow))
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"count": gjson.GetBytes(body, "totalElements").Int(),
		}, nil
	})

	probe("vendors", func() (any, error) {
		body, err := c.Get(ctx, "/api/inventory/v2/inventories/vendor", paramsWith("size", "1"))
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"total": gjson.GetBytes(body, "totalElements").Int(),
		}, nil
	})

	probe("webhooks", func() (any, error) {
		body, err := c.Get(ctx, "/api/webhook/v1/subscriptions", nil)
		if err != nil {
			return nil, err
		}
		items := flattenItems(body)
		var arr []any
		_ = json.Unmarshal(items, &arr)
		return map[string]any{"subscriptions": len(arr)}, nil
	})

	probe("auth", func() (any, error) {
		info, err := c.Tokens().PeekToken(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"expires_in_seconds": info.ExpiresIn}, nil
	})

	probe("scopes", func() (any, error) {
		body, err := c.Get(ctx, "/api/access/v1/oauth/scopes", nil)
		if err != nil {
			return nil, err
		}
		items := flattenItems(body)
		var arr []any
		_ = json.Unmarshal(items, &arr)
		return map[string]any{"granted": len(arr)}, nil
	})

	wg.Wait()
	return printJSONValue(result)
}

// sanitizeError converts an APIError into a small map suitable for embedding
// inside the overview response.
func sanitizeError(err error) map[string]any {
	if apiErr, ok := err.(interface{ Error() string }); ok {
		return map[string]any{"detail": apiErr.Error()}
	}
	return map[string]any{"detail": err.Error()}
}
