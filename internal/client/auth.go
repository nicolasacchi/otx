package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenProvider holds OAuth 2.0 client-credentials and caches the latest bearer.
// Tokens are refreshed when the cached value is within refreshSkew of expiry, or
// when an upstream call returns 401.
type TokenProvider struct {
	baseURL      string
	clientID     string
	clientSecret string
	http         *http.Client
	verbose      bool

	// staticToken, when non-empty, is a long-lived OneTrust API key used
	// directly as the bearer. No OAuth token exchange is performed.
	staticToken string

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

const (
	tokenPath    = "/api/access/v1/oauth/token"
	refreshSkew  = 60 * time.Second
	tokenTimeout = 15 * time.Second
)

// NewTokenProvider constructs an OAuth provider; baseURL is the OneTrust tenant
// origin (e.g. "https://app-eu.onetrust.com"). Token is lazily fetched on first
// Get().
func NewTokenProvider(baseURL, clientID, clientSecret string, verbose bool) *TokenProvider {
	return &TokenProvider{
		baseURL:      strings.TrimRight(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: tokenTimeout},
		verbose:      verbose,
	}
}

// NewStaticTokenProvider builds a provider that uses a long-lived OneTrust API
// key directly as the bearer token — no OAuth client-credentials exchange. The
// key is the value generated under Global Settings → Access Management → API
// Keys; it is sent verbatim as "Authorization: Bearer <key>".
func NewStaticTokenProvider(baseURL, apiKey string, verbose bool) *TokenProvider {
	return &TokenProvider{
		baseURL:     strings.TrimRight(baseURL, "/"),
		staticToken: apiKey,
		http:        &http.Client{Timeout: tokenTimeout},
		verbose:     verbose,
	}
}

// IsStatic reports whether this provider serves a long-lived API key rather
// than exchanging OAuth client credentials.
func (p *TokenProvider) IsStatic() bool { return p.staticToken != "" }

// Get returns a valid bearer token, refreshing if expired or within
// refreshSkew of expiry.
func (p *TokenProvider) Get(ctx context.Context) (string, error) {
	if p.staticToken != "" {
		return p.staticToken, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Until(p.expiresAt) > refreshSkew {
		return p.token, nil
	}
	return p.refreshLocked(ctx)
}

// Invalidate forces the next Get() to refresh. Called by the HTTP client on 401.
// For a static API key there is nothing to refresh, so it is a no-op.
func (p *TokenProvider) Invalidate() {
	if p.staticToken != "" {
		return
	}
	p.mu.Lock()
	p.token = ""
	p.expiresAt = time.Time{}
	p.mu.Unlock()
}

// refreshLocked performs the actual token exchange. Caller must hold p.mu.
func (p *TokenProvider) refreshLocked(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+tokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return "", &APIError{Kind: "auth_failed", Detail: fmt.Sprintf("build token request: %v", err), Hint: hintForStatus(401)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", &APIError{Kind: "auth_failed", Detail: fmt.Sprintf("token request failed: %v", err), Hint: "verify OTX_BASE_URL reachability"}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return "", &APIError{
			Status: resp.StatusCode,
			Kind:   "auth_failed",
			Detail: fmt.Sprintf("token exchange rejected: %s", detail),
			Hint:   hintForStatus(resp.StatusCode),
		}
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", &APIError{Kind: "auth_failed", Detail: fmt.Sprintf("parse token response: %v", err)}
	}
	if tok.AccessToken == "" {
		return "", &APIError{Kind: "auth_failed", Detail: "OneTrust returned an empty access_token"}
	}

	p.token = tok.AccessToken
	if tok.ExpiresIn > 0 {
		p.expiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	} else {
		// Conservative default — most OneTrust tokens live for ~1h.
		p.expiresAt = time.Now().Add(55 * time.Minute)
	}
	return p.token, nil
}

// TokenInfo exposes the cached token state for `otx auth token` commands.
type TokenInfo struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	ExpiresIn   int       `json:"expires_in_seconds"`
}

// PeekToken returns a TokenInfo snapshot, refreshing if necessary.
func (p *TokenProvider) PeekToken(ctx context.Context) (*TokenInfo, error) {
	if p.staticToken != "" {
		// Static API key — no exchange, no known expiry (ExpiresIn -1 = "n/a").
		return &TokenInfo{AccessToken: p.staticToken, ExpiresIn: -1}, nil
	}
	tok, err := p.Get(ctx)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	exp := p.expiresAt
	p.mu.Unlock()
	return &TokenInfo{
		AccessToken: tok,
		ExpiresAt:   exp,
		ExpiresIn:   int(time.Until(exp).Seconds()),
	}, nil
}
