package output

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
)

// ApplyFilter applies a gjson path expression to JSON data. Returns json.null
// when the path doesn't match; returns an error only when gjson produces a
// non-JSON value (e.g. a malformed projection).
func ApplyFilter(data json.RawMessage, path string) (json.RawMessage, error) {
	if path == "" {
		return data, nil
	}
	result := gjson.GetBytes(data, path)
	if !result.Exists() {
		return json.RawMessage("null"), nil
	}
	raw := result.Raw
	if raw == "" {
		return nil, fmt.Errorf("gjson filter %q returned non-JSON result", path)
	}
	return json.RawMessage(raw), nil
}
