package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/tidwall/gjson"
)

// scopesUnavailable reports whether an error from the OneTrust scope-discovery
// endpoint (/api/access/v1/oauth/scopes) means the endpoint is simply absent on
// this tenant — some OneTrust versions/regions don't expose it — rather than a
// genuine failure. Callers use this to degrade gracefully instead of hard-failing.
func scopesUnavailable(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Kind == "not_found" || apiErr.Kind == "non_json_response"
	}
	return false
}

// renderScopesUnavailable prints a clean, exit-0 envelope explaining that the
// tenant does not expose the scope-discovery endpoint.
func renderScopesUnavailable() error {
	return printJSONValue(map[string]any{
		"ok":        false,
		"available": false,
		"detail":    "OneTrust scope-discovery endpoint (/api/access/v1/oauth/scopes) is not available on this tenant",
		"hint":      "inspect granted scopes in the OneTrust UI: Global Settings → Access Management → Client Credentials",
	})
}

// flattenItems normalizes a OneTrust paged response (which may use
// "content"/"data"/"Resources"/"items") into a JSON array. When the body is
// already an array it is returned unchanged.
func flattenItems(body json.RawMessage) json.RawMessage {
	root := gjson.ParseBytes(body)
	if root.IsArray() {
		return body
	}
	for _, k := range []string{"content", "data", "Resources", "items", "value"} {
		v := root.Get(k)
		if v.IsArray() {
			return json.RawMessage(v.Raw)
		}
	}
	return body
}

// paramsWith builds a url.Values seeded with optional key/value string pairs;
// empty values are skipped.
func paramsWith(kv ...string) url.Values {
	p := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			continue
		}
		p.Set(kv[i], kv[i+1])
	}
	return p
}

// scimPaginateOpts builds a SCIM pagination options struct from a limit value.
func scimPaginateOpts(limit int) clientSCIMOpts {
	return clientSCIMOpts{Count: 50, MaxItems: limit}
}

// buildMultipart reads a file from disk and packages it as a multipart/form-data
// body with field name "file". Returns the body bytes and the Content-Type
// (which includes the random boundary) so the caller can hand both to
// client.Raw.
func buildMultipart(path string) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}
