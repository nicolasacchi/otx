package commands

import (
	"encoding/json"
	"net/url"

	"github.com/tidwall/gjson"
)

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
