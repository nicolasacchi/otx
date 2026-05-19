package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/tidwall/gjson"
	"golang.org/x/term"
)

// AgentMode returns true when running under Claude Code (CLAUDECODE=1). When
// true, default --limit caps tighten and truncated:true markers are appended.
func AgentMode() bool {
	v := os.Getenv("CLAUDECODE")
	return v != "" && v != "0" && v != "false"
}

// IsJSON returns true if output should be JSON (non-TTY, --json flag, or --jq set).
func IsJSON(jsonFlag bool, jqFilter string) bool {
	if jsonFlag || jqFilter != "" {
		return true
	}
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

// PrintData outputs data as JSON or table depending on context. The optional
// truncated marker is appended into top-level JSON objects (not arrays) when
// the result has been row-capped under CLAUDECODE.
func PrintData(command string, data json.RawMessage, jsonMode bool, jqFilter string, truncated bool) error {
	if !jsonMode {
		if err := PrintTable(command, data); err == nil {
			return nil
		}
	}

	if jqFilter != "" {
		filtered, err := ApplyFilter(data, jqFilter)
		if err != nil {
			return err
		}
		data = filtered
	}

	if truncated {
		data = appendTruncatedMarker(data)
	}
	return printJSON(data)
}

// PrintError outputs the structured error envelope to stdout. Stderr lines
// (human message + hint) are written separately by main.go.
func PrintError(detail string, kind string, status int, hint string) {
	envelope := map[string]any{
		"ok": false,
		"error": map[string]any{
			"kind":   kind,
			"status": status,
			"detail": detail,
		},
	}
	if hint != "" {
		envelope["error"].(map[string]any)["hint"] = hint
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(envelope)
}

func printJSON(data json.RawMessage) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}
	return nil
}

// PrintJSONValue prints any Go value as formatted JSON.
func PrintJSONValue(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}
	return nil
}

// ResolveLimit returns the effective --limit value. When CLAUDECODE is set and
// the user hasn't lifted the cap with --rows 0, the limit is bounded to
// agentCap. Returns (effective, capped) where capped indicates the original
// flag value was reduced.
func ResolveLimit(requested int, agentCap int, agentOverride int) (int, bool) {
	if !AgentMode() || agentOverride != -1 {
		if agentOverride > 0 {
			return agentOverride, false
		}
		return requested, false
	}
	if requested <= 0 || requested > agentCap {
		return agentCap, requested > agentCap
	}
	return requested, false
}

func appendTruncatedMarker(data json.RawMessage) json.RawMessage {
	root := gjson.ParseBytes(data)
	if !root.IsObject() {
		return data
	}
	// Trim trailing brace and append marker.
	raw := []byte(data)
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] == '}' {
			prefix := raw[:i]
			suffix := raw[i:]
			marker := []byte(",\"truncated\":true")
			// Avoid double-comma if prefix ends with "{" (empty object).
			trim := []byte{}
			for j := len(prefix) - 1; j >= 0; j-- {
				if prefix[j] == ' ' || prefix[j] == '\t' || prefix[j] == '\n' || prefix[j] == '\r' {
					continue
				}
				if prefix[j] == '{' {
					marker = []byte("\"truncated\":true")
				}
				break
			}
			out := append([]byte{}, prefix...)
			out = append(out, trim...)
			out = append(out, marker...)
			out = append(out, suffix...)
			return out
		}
	}
	return data
}

// Itoa is a tiny strconv wrapper used by command files to keep imports lean.
func Itoa(n int) string { return strconv.Itoa(n) }
