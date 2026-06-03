package client

import "fmt"

// APIError represents a structured error from the OneTrust API or from otx
// internals (auth failures, write-locked operations, etc).
//
// Kind is a stable symbolic identifier suitable for agent dispatch. Status
// is the HTTP status code when the error originated from a remote call (0 for
// local errors). Hint is an actionable single-line suggestion shown to humans
// on stderr.
type APIError struct {
	Status int
	Kind   string
	Detail string
	Hint   string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		if e.Status > 0 {
			return fmt.Sprintf("%d: %s", e.Status, e.Detail)
		}
		return e.Detail
	}
	if e.Status > 0 {
		return fmt.Sprintf("API error %d", e.Status)
	}
	return "error"
}

// ExitCode maps the error to the documented otx exit-code table.
//
//	0 ok
//	1 generic
//	2 auth          (401, 403)
//	3 validation    (400)
//	4 not-found     (404)
//	5 rate-limit    (429)
//	6 deprecated / unsupported / write-locked
//	7 async-timeout
func (e *APIError) ExitCode() int {
	switch e.Kind {
	case "auth_failed", "forbidden_scope":
		return 2
	case "validation":
		return 3
	case "not_found", "non_json_response":
		return 4
	case "rate_limited":
		return 5
	case "deprecated_endpoint", "not_publicly_documented", "write_locked":
		return 6
	case "async_timeout":
		return 7
	}
	switch e.Status {
	case 401, 403:
		return 2
	case 400:
		return 3
	case 404:
		return 4
	case 429:
		return 5
	}
	return 1
}

func kindForStatus(status int) string {
	switch status {
	case 400:
		return "validation"
	case 401:
		return "auth_failed"
	case 403:
		return "forbidden_scope"
	case 404:
		return "not_found"
	case 429:
		return "rate_limited"
	}
	if status >= 500 {
		return "server_error"
	}
	return "api_error"
}

func hintForStatus(status int) string {
	switch status {
	case 400:
		return "request body or query rejected — check field shapes and required params"
	case 401:
		return "auth failed — check client_id/client_secret or run 'otx config doctor'"
	case 403:
		return "missing OAuth scope — check 'otx auth scopes list' against this endpoint's requirement"
	case 404:
		return "resource not found — verify the ID and that your tenant has the module enabled"
	case 429:
		return "rate limited — backoff or contact OneTrust support for limits"
	}
	if status >= 500 {
		return "OneTrust server error — retry after a brief delay; check https://status.onetrust.com/"
	}
	return ""
}
