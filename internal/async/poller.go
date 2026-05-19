// Package async polls long-running OneTrust jobs (DSAR fulfilment, data
// discovery scans, bulk exports) until they reach a terminal state.
package async

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/tidwall/gjson"
)

// Options configures Wait.
type Options struct {
	Interval       time.Duration // poll cadence; default 5s
	Timeout        time.Duration // hard cap; default 10m. Zero means no cap.
	TerminalStates []string      // states that end polling. Matched case-insensitively.
	StatusField    string        // gjson path to the status string; default "status"
	OnPoll         func(snapshot json.RawMessage)
}

// Wait polls jobURL (must be absolute or tenant-relative) until the response's
// status matches one of TerminalStates, the timeout fires, or the context is
// cancelled.
func Wait(ctx context.Context, c *client.Client, jobURL string, opts Options) (json.RawMessage, error) {
	if opts.Interval == 0 {
		opts.Interval = 5 * time.Second
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Minute
	}
	if opts.StatusField == "" {
		opts.StatusField = "status"
	}
	if len(opts.TerminalStates) == 0 {
		opts.TerminalStates = []string{"COMPLETED", "COMPLETE", "FAILED", "ERROR", "EXPIRED", "CANCELLED"}
	}

	deadline := time.Now().Add(opts.Timeout)
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	// First poll immediately.
	for {
		body, err := c.Get(ctx, jobURL, nil)
		if err != nil {
			return nil, err
		}
		if opts.OnPoll != nil {
			opts.OnPoll(body)
		}
		state := gjson.GetBytes(body, opts.StatusField).String()
		for _, term := range opts.TerminalStates {
			if equalFold(state, term) {
				return body, nil
			}
		}

		if opts.Timeout > 0 && time.Now().After(deadline) {
			return body, &client.APIError{
				Kind:   "async_timeout",
				Detail: fmt.Sprintf("job %s did not reach a terminal state within %s (last state: %q)", jobURL, opts.Timeout, state),
				Hint:   "rerun with --timeout, or check the OneTrust UI for the actual job state",
			}
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
