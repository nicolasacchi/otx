// Package timeparse converts user-supplied time strings into Unix milliseconds.
// Accepted forms:
//
//	"" / "now"      → current time
//	"5m" / "1h"      → relative past offset
//	"now-2h"         → relative past offset (explicit form)
//	RFC3339          → "2026-05-19T10:00:00Z"
//	epoch (seconds)  → "1747652400"
//	epoch (millis)   → "1747652400000"
package timeparse

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseMillis returns Unix milliseconds for the given expression.
func ParseMillis(expr string) (int64, error) {
	t, err := Parse(expr)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

// Parse returns a time.Time for the given expression.
func Parse(expr string) (time.Time, error) {
	now := time.Now().UTC()
	s := strings.TrimSpace(expr)
	if s == "" || s == "now" {
		return now, nil
	}
	if strings.HasPrefix(s, "now-") {
		d, err := time.ParseDuration(strings.TrimPrefix(s, "now-"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration in %q: %w", s, err)
		}
		return now.Add(-d), nil
	}
	if strings.HasPrefix(s, "now+") {
		d, err := time.ParseDuration(strings.TrimPrefix(s, "now+"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration in %q: %w", s, err)
		}
		return now.Add(d), nil
	}
	// Bare durations interpreted as past offset.
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	// Day-precision duration (Go's time.ParseDuration doesn't accept "d").
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return now.Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	// Try RFC3339 (with and without sub-second).
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	// Epoch.
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		switch {
		case v > 1e12: // millis
			return time.UnixMilli(v).UTC(), nil
		case v > 1e9: // seconds
			return time.Unix(v, 0).UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time expression: %q", expr)
}
