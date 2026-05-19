package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	MaxRetries = 3
	Timeout    = 60 * time.Second
)

// Client is the OneTrust REST client. It uses an injected TokenProvider for
// OAuth 2.0 client-credentials bearer tokens, refreshes on 401, and retries on
// 429 / 5xx with exponential backoff + jitter (honoring Retry-After).
type Client struct {
	http       *http.Client
	tokens     *TokenProvider
	baseURL    string
	verbose    bool
	maxRetries int
}

// New constructs a client. baseURL is the OneTrust tenant origin
// (e.g. "https://app-eu.onetrust.com").
func New(baseURL string, tokens *TokenProvider, verbose bool) *Client {
	return &Client{
		http:       &http.Client{Timeout: Timeout},
		tokens:     tokens,
		baseURL:    strings.TrimRight(baseURL, "/"),
		verbose:    verbose,
		maxRetries: MaxRetries,
	}
}

// BaseURL exposes the configured origin (used by commands that need to build
// extra-tenant URLs, e.g. /pdf export links).
func (c *Client) BaseURL() string { return c.baseURL }

// Tokens exposes the underlying provider (used by `otx auth token`).
func (c *Client) Tokens() *TokenProvider { return c.tokens }

// Get performs a GET request and returns the response body as raw JSON.
func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	return c.doRequest(ctx, http.MethodGet, c.buildURL(path, params), nil)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doJSON(ctx, http.MethodPost, c.buildURL(path, nil), body)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doJSON(ctx, http.MethodPut, c.buildURL(path, nil), body)
}

// Patch performs a PATCH request with a JSON body.
func (c *Client) Patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doJSON(ctx, http.MethodPatch, c.buildURL(path, nil), body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, c.buildURL(path, nil), nil)
	return err
}

// Raw performs a request with an explicit method and an optional raw body. The
// content-type header is set when contentType != "".
func (c *Client) Raw(ctx context.Context, method, path string, contentType string, body []byte) (json.RawMessage, error) {
	rawURL := c.buildURL(path, nil)
	return c.doRequestWithBody(ctx, method, rawURL, body, contentType)
}

func (c *Client) buildURL(path string, params url.Values) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if len(params) > 0 {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			return path + sep + params.Encode()
		}
		return path
	}
	u := c.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u
}

func (c *Client) doJSON(ctx context.Context, method, rawURL string, body any) (json.RawMessage, error) {
	var bodyBytes []byte
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		bodyBytes = buf.Bytes()
	}
	return c.doRequestWithBody(ctx, method, rawURL, bodyBytes, "application/json")
}

func (c *Client) doRequest(ctx context.Context, method, rawURL string, body io.Reader) (json.RawMessage, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}
	return c.doRequestWithBody(ctx, method, rawURL, bodyBytes, "")
}

func (c *Client) doRequestWithBody(ctx context.Context, method, rawURL string, bodyBytes []byte, contentType string) (json.RawMessage, error) {
	if c.verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, rawURL)
		if len(bodyBytes) > 0 && len(bodyBytes) < 4096 {
			fmt.Fprintf(os.Stderr, "> Body: %s\n", string(bodyBytes))
		} else if len(bodyBytes) > 0 {
			fmt.Fprintf(os.Stderr, "> Body: <%d bytes>\n", len(bodyBytes))
		}
	}

	start := time.Now()
	resp, err := c.doWithRetry(ctx, method, rawURL, bodyBytes, contentType)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if c.verbose {
		fmt.Fprintf(os.Stderr, "< %d %s (%s, %s)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed.Round(time.Millisecond), humanBytes(len(respBody)))
	}

	if resp.StatusCode == http.StatusNoContent {
		return json.RawMessage("{}"), nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if c.verbose && len(respBody) > 0 && len(respBody) < 4096 {
			fmt.Fprintf(os.Stderr, "< Body: %s\n", string(respBody))
		}
		return nil, parseAPIError(respBody, resp.StatusCode)
	}

	if len(respBody) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(respBody), nil
}

func (c *Client) doWithRetry(ctx context.Context, method, rawURL string, bodyBytes []byte, contentType string) (*http.Response, error) {
	var lastErr error
	retriedAuth := false

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var bodyReader io.Reader
		if len(bodyBytes) > 0 {
			bodyReader = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		token, err := c.tokens.Get(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if contentType != "" && len(bodyBytes) > 0 {
			req.Header.Set("Content-Type", contentType)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt == c.maxRetries {
				return nil, lastErr
			}
			delay := retryDelay(attempt, "")
			if c.verbose {
				fmt.Fprintf(os.Stderr, "! request error, retrying in %s (attempt %d/%d)\n", delay, attempt+1, c.maxRetries)
			}
			if err := sleepCtx(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		// Refresh-once on 401 in case the cached bearer is stale.
		if resp.StatusCode == http.StatusUnauthorized && !retriedAuth {
			resp.Body.Close()
			retriedAuth = true
			c.tokens.Invalidate()
			if c.verbose {
				fmt.Fprintln(os.Stderr, "! 401 — refreshing token and retrying once")
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt == c.maxRetries {
				return nil, &APIError{
					Status: resp.StatusCode,
					Kind:   kindForStatus(resp.StatusCode),
					Detail: fmt.Sprintf("failed after %d retries", c.maxRetries),
					Hint:   hintForStatus(resp.StatusCode),
				}
			}
			delay := retryDelay(attempt, resp.Header.Get("Retry-After"))
			if c.verbose {
				fmt.Fprintf(os.Stderr, "! %d %s, retrying in %s (attempt %d/%d)\n",
					resp.StatusCode, http.StatusText(resp.StatusCode), delay, attempt+1, c.maxRetries)
			}
			if err := sleepCtx(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		return resp, nil
	}
	return nil, lastErr
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func parseAPIError(body []byte, statusCode int) *APIError {
	apiErr := &APIError{
		Status: statusCode,
		Kind:   kindForStatus(statusCode),
		Hint:   hintForStatus(statusCode),
	}

	var generic map[string]any
	if json.Unmarshal(body, &generic) == nil {
		// OneTrust returns several error shapes. Try the documented ones in order.
		// 1. {"message": "..."}
		if msg, ok := generic["message"].(string); ok && msg != "" {
			apiErr.Detail = msg
		}
		// 2. {"error": "...", "error_description": "..."} (OAuth-style)
		if desc, ok := generic["error_description"].(string); ok && desc != "" {
			apiErr.Detail = desc
		} else if ec, ok := generic["error"].(string); ok && ec != "" && apiErr.Detail == "" {
			apiErr.Detail = ec
		}
		// 3. {"detail": "..."}
		if d, ok := generic["detail"].(string); ok && d != "" && apiErr.Detail == "" {
			apiErr.Detail = d
		}
		// 4. {"errors": [{"message": "..."}]}
		if errs, ok := generic["errors"].([]any); ok && len(errs) > 0 {
			if obj, ok := errs[0].(map[string]any); ok {
				if d, ok := obj["message"].(string); ok && d != "" && apiErr.Detail == "" {
					apiErr.Detail = d
				} else if d, ok := obj["detail"].(string); ok && d != "" && apiErr.Detail == "" {
					apiErr.Detail = d
				}
			} else if s, ok := errs[0].(string); ok && apiErr.Detail == "" {
				apiErr.Detail = s
			}
		}
		// 5. SCIM: {"schemas":["...:Error"], "detail": "...", "scimType": "..."}
		if scimType, ok := generic["scimType"].(string); ok && scimType != "" && apiErr.Detail == "" {
			apiErr.Detail = scimType
		}
	}

	if apiErr.Detail == "" {
		apiErr.Detail = strings.TrimSpace(string(body))
		if apiErr.Detail == "" {
			apiErr.Detail = http.StatusText(statusCode)
		}
	}
	return apiErr
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if t, err := http.ParseTime(retryAfter); err == nil {
			d := time.Until(t)
			if d > 0 {
				return d
			}
		}
	}
	base := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	jitter := time.Duration(rand.IntN(500)) * time.Millisecond
	return base + jitter
}

func humanBytes(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	kb := float64(b) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1fKB", kb)
	}
	return fmt.Sprintf("%.1fMB", kb/1024)
}
