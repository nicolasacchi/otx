package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsNonJSONBody(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
		want        bool
	}{
		{"html content-type", "text/html; charset=utf-8", "<!DOCTYPE html>", true},
		{"xml content-type", "application/xml", "<note/>", true},
		{"html sniffed despite json ct", "application/json", "  \n<!DOCTYPE html>", true},
		{"json object", "application/json", `{"a":1}`, false},
		{"json array", "application/json", `[1,2,3]`, false},
		{"problem+json error", "application/problem+json", `{"title":"Not Found"}`, false},
		{"leading whitespace json", "", "  \n\t{\"a\":1}", false},
		{"empty", "application/json", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNonJSONBody(tc.contentType, []byte(tc.body)); got != tc.want {
				t.Fatalf("isNonJSONBody(%q, %q) = %v, want %v", tc.contentType, tc.body, got, tc.want)
			}
		})
	}
}

func TestStaticTokenProvider(t *testing.T) {
	tp := NewStaticTokenProvider("https://app-de.onetrust.com", "APIKEY123", false)
	if !tp.IsStatic() {
		t.Fatal("IsStatic() = false, want true")
	}
	got, err := tp.Get(context.Background())
	if err != nil || got != "APIKEY123" {
		t.Fatalf("Get() = %q, %v; want APIKEY123, nil", got, err)
	}
	tp.Invalidate() // must be a no-op for a static key
	if got, _ := tp.Get(context.Background()); got != "APIKEY123" {
		t.Fatalf("Get() after Invalidate() = %q, want APIKEY123", got)
	}
	info, err := tp.PeekToken(context.Background())
	if err != nil || info.AccessToken != "APIKEY123" || info.ExpiresIn != -1 {
		t.Fatalf("PeekToken() = %+v, %v; want {APIKEY123, ExpiresIn:-1}, nil", info, err)
	}
}

func TestParseAPIErrorDoesNotLeakHTML(t *testing.T) {
	html := "<!DOCTYPE html><html><body>Access denied</body></html>"
	err := parseAPIError([]byte(html), http.StatusForbidden)
	if err.Kind != "forbidden_scope" {
		t.Fatalf("kind = %q, want forbidden_scope", err.Kind)
	}
	if strings.Contains(err.Detail, "<") {
		t.Fatalf("detail leaked HTML: %q", err.Detail)
	}
	if err.Detail != http.StatusText(http.StatusForbidden) {
		t.Fatalf("detail = %q, want %q", err.Detail, http.StatusText(http.StatusForbidden))
	}
}

// newTestClient wires a Client + TokenProvider at the given test server, with
// the token endpoint served by the same server.
func newTestClient(t *testing.T, base string) *Client {
	t.Helper()
	tp := NewTokenProvider(base, "cid", "secret", false)
	return New(base, tp, false)
}

func TestGetRejectsHTMLShellWith200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/access/v1/oauth/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/webhook/v1/subscriptions":
			// OneTrust's SPA shell: 200 + text/html for an unmatched API route.
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<!DOCTYPE html><html><head></head></html>"))
		case "/api/scim/v2/Users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Resources":[{"id":"1"}]}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	// HTML shell on a 200 must become a clean non_json_response error.
	_, err := c.Get(context.Background(), "/api/webhook/v1/subscriptions", nil)
	if err == nil {
		t.Fatal("expected error for HTML 200 response, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Kind != "non_json_response" {
		t.Fatalf("kind = %q, want non_json_response", apiErr.Kind)
	}
	if apiErr.ExitCode() != 4 {
		t.Fatalf("exit code = %d, want 4", apiErr.ExitCode())
	}

	// A genuine JSON body still passes through untouched.
	body, err := c.Get(context.Background(), "/api/scim/v2/Users", nil)
	if err != nil {
		t.Fatalf("unexpected error on JSON endpoint: %v", err)
	}
	if !strings.Contains(string(body), `"Resources"`) {
		t.Fatalf("unexpected JSON body: %s", body)
	}
}
