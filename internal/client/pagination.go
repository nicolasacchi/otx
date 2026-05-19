package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/tidwall/gjson"
)

// PageOptions controls page/size pagination over standard OneTrust v1/v2 list
// endpoints. Most OneTrust list endpoints return JSON of the shape:
//
//	{ "content": [...], "totalElements": N, "totalPages": M, "page": 0, "size": 50 }
//
// or:
//
//	{ "data": [...], "page": 0, "size": 50, "totalResults": N }
//
// PaginatePage normalizes both shapes into a flat []json.RawMessage by walking
// pages until totalPages is reached or maxItems is hit.
type PageOptions struct {
	StartPage int    // default 0
	Size      int    // page size; default 50
	MaxItems  int    // hard cap (0 = unlimited)
	Param     string // override page param name (default "page")
	SizeParam string // override size param name (default "size")
	ItemsKey  string // override JSON key used to find the items array; if empty, "content" → "data" → "Resources" are tried
}

// PaginatePage walks page-based pagination on a GET endpoint. extraParams are
// merged into each page request.
func (c *Client) PaginatePage(ctx context.Context, path string, extraParams url.Values, opts PageOptions) ([]json.RawMessage, error) {
	if opts.Size <= 0 {
		opts.Size = 50
	}
	if opts.Param == "" {
		opts.Param = "page"
	}
	if opts.SizeParam == "" {
		opts.SizeParam = "size"
	}

	out := make([]json.RawMessage, 0, opts.Size)
	page := opts.StartPage

	for {
		params := url.Values{}
		for k, v := range extraParams {
			params[k] = v
		}
		params.Set(opts.Param, strconv.Itoa(page))
		params.Set(opts.SizeParam, strconv.Itoa(opts.Size))

		body, err := c.Get(ctx, path, params)
		if err != nil {
			return out, err
		}

		items, totalPages := extractItems(body, opts.ItemsKey)
		for _, it := range items {
			out = append(out, it)
			if opts.MaxItems > 0 && len(out) >= opts.MaxItems {
				return out, nil
			}
		}

		page++
		if totalPages > 0 && page >= totalPages {
			return out, nil
		}
		if len(items) < opts.Size {
			return out, nil
		}
	}
}

// SCIMOptions controls startIndex/count pagination for SCIM 2.0 endpoints
// (/api/scim/v2/Users, /Groups).
type SCIMOptions struct {
	StartIndex int // SCIM is 1-based; default 1
	Count      int // page size; default 50
	MaxItems   int // hard cap (0 = unlimited)
}

// PaginateSCIM walks SCIM startIndex/count pagination.
func (c *Client) PaginateSCIM(ctx context.Context, path string, extraParams url.Values, opts SCIMOptions) ([]json.RawMessage, error) {
	if opts.StartIndex <= 0 {
		opts.StartIndex = 1
	}
	if opts.Count <= 0 {
		opts.Count = 50
	}

	out := make([]json.RawMessage, 0, opts.Count)
	idx := opts.StartIndex

	for {
		params := url.Values{}
		for k, v := range extraParams {
			params[k] = v
		}
		params.Set("startIndex", strconv.Itoa(idx))
		params.Set("count", strconv.Itoa(opts.Count))

		body, err := c.Get(ctx, path, params)
		if err != nil {
			return out, err
		}

		total := int(gjson.GetBytes(body, "totalResults").Int())
		resources := gjson.GetBytes(body, "Resources")
		if !resources.IsArray() {
			return out, fmt.Errorf("SCIM response missing Resources array")
		}
		resources.ForEach(func(_, value gjson.Result) bool {
			out = append(out, json.RawMessage(value.Raw))
			if opts.MaxItems > 0 && len(out) >= opts.MaxItems {
				return false
			}
			return true
		})
		if opts.MaxItems > 0 && len(out) >= opts.MaxItems {
			return out, nil
		}

		idx += opts.Count
		if total > 0 && idx > total {
			return out, nil
		}
		if int(resources.Get("#").Int()) < opts.Count {
			return out, nil
		}
	}
}

func extractItems(body json.RawMessage, key string) (items []json.RawMessage, totalPages int) {
	candidates := []string{key}
	if key == "" {
		candidates = []string{"content", "data", "Resources", "items"}
	}
	for _, k := range candidates {
		if k == "" {
			continue
		}
		arr := gjson.GetBytes(body, k)
		if arr.IsArray() {
			arr.ForEach(func(_, value gjson.Result) bool {
				items = append(items, json.RawMessage(value.Raw))
				return true
			})
			totalPages = int(gjson.GetBytes(body, "totalPages").Int())
			return items, totalPages
		}
	}
	// Last resort: body itself is an array.
	if gjson.ParseBytes(body).IsArray() {
		gjson.ParseBytes(body).ForEach(func(_, value gjson.Result) bool {
			items = append(items, json.RawMessage(value.Raw))
			return true
		})
	}
	return items, 0
}
