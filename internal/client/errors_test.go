package client

import "testing"

// TestAPIError_ExitCode pins the full otx exit-code table after the refactor that
// delegates the status fallback + shared guard kinds to clicore's ExitCodeFor.
// Both the otx-specific kinds and the HTTP-status fallback must keep their codes.
func TestAPIError_ExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  APIError
		want int
	}{
		// otx-specific kinds (must NOT regress to the generic table)
		{"kind auth_failed", APIError{Kind: "auth_failed"}, 2},
		{"kind forbidden_scope", APIError{Kind: "forbidden_scope"}, 2},
		{"kind validation (Status 0, local guard)", APIError{Kind: "validation"}, 3},
		{"kind not_found", APIError{Kind: "not_found"}, 4},
		{"kind non_json_response (SPA shell)", APIError{Kind: "non_json_response", Status: 200}, 4},
		{"kind rate_limited", APIError{Kind: "rate_limited"}, 5},
		// shared guard kinds (delegated to clicore)
		{"kind write_locked", APIError{Kind: "write_locked"}, 6},
		{"kind deprecated_endpoint", APIError{Kind: "deprecated_endpoint"}, 6},
		{"kind not_publicly_documented", APIError{Kind: "not_publicly_documented"}, 6},
		{"kind async_timeout", APIError{Kind: "async_timeout"}, 7},
		// HTTP-status fallback (no/unknown kind)
		{"status 401", APIError{Status: 401}, 2},
		{"status 403", APIError{Status: 403}, 2},
		{"status 400", APIError{Status: 400}, 3},
		{"status 404", APIError{Status: 404}, 4},
		{"status 429", APIError{Status: 429}, 5},
		{"status 500", APIError{Status: 500}, 1},
		{"status 502", APIError{Status: 502}, 1},
		{"empty (generic)", APIError{}, 1},
		{"unknown kind falls back to status", APIError{Kind: "server_error", Status: 503}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.ExitCode(); got != tc.want {
				t.Errorf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}
