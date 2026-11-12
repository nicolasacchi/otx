package commands

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nicolasacchi/otx/internal/client"
)

func TestParseExportDate(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid", "2026-08-01", false},
		{"wrong format", "08/01/2026", true},
		{"garbage", "not-a-date", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseExportDate("from", tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseExportDate(%q) err = %v, wantErr %v", tc.value, err, tc.wantErr)
			}
		})
	}
}

func TestValidateExportWindow(t *testing.T) {
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().UTC().Format("2006-01-02")
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	// Anchor "from" cases relative to yesterday, not a hardcoded date, so this
	// test doesn't depend on when it happens to run.
	sixtyDaysBeforeYesterday := time.Now().UTC().AddDate(0, 0, -61).Format("2006-01-02")
	twoHundredDaysBeforeYesterday := time.Now().UTC().AddDate(0, 0, -201).Format("2006-01-02")
	exactly180DaysBeforeYesterday := time.Now().UTC().AddDate(0, 0, -181).Format("2006-01-02")

	cases := []struct {
		name       string
		from, to   string
		wantErr    bool
		errmsgPart string
	}{
		{"missing from", "", yesterday, true, "--from"},
		{"missing to", sixtyDaysBeforeYesterday, "", true, "--to"},
		{"to before from", "2026-08-01", "2026-07-01", true, "before"},
		{"to is today", sixtyDaysBeforeYesterday, today, true, "yesterday"},
		{"to is future", sixtyDaysBeforeYesterday, tomorrow, true, "yesterday"},
		{"to is yesterday, ok", sixtyDaysBeforeYesterday, yesterday, false, ""},
		{"span over 180 days", twoHundredDaysBeforeYesterday, yesterday, true, "180-day"},
		{"span exactly within 180 days", exactly180DaysBeforeYesterday, yesterday, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := validateExportWindow(tc.from, tc.to)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateExportWindow(%q, %q) err = %v, wantErr %v", tc.from, tc.to, err, tc.wantErr)
			}
			if tc.wantErr && tc.errmsgPart != "" && !strings.Contains(err.Error(), tc.errmsgPart) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.errmsgPart)
			}
		})
	}
}

func TestExtractExportJobID(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"runId preferred", `{"runId":"r1","id":"i1","exportId":"e1"}`, "r1"},
		{"falls back to id", `{"id":"i1","exportId":"e1"}`, "i1"},
		{"falls back to exportId", `{"exportId":"e1"}`, "e1"},
		{"absent entirely (unverified create shape)", `{"errors":[{"code":"ERROR_OT_INSIGHTS_COMMAND_FEATURE_NOT_ENABLED"}]}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractExportJobID(json.RawMessage(tc.body))
			if got != tc.want {
				t.Fatalf("extractExportJobID(%s) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

// TestFirstDownloadLink verifies the array-shaped download response (the
// live-observed shape) is parsed correctly and that an object-shaped or
// empty-array response fails clearly instead of panicking.
func TestFirstDownloadLink(t *testing.T) {
	arrayBody := `[{"id":"9e9bc2ff-1","headers":{"Authorization":"Bearer shortlived"},"url":"https://app-de.onetrust.com/api/document-gateway/9e9bc2ff-1","fileName":"9e9bc2ff-1.csv.gz"}]`
	link, err := firstDownloadLink(json.RawMessage(arrayBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.URL != "https://app-de.onetrust.com/api/document-gateway/9e9bc2ff-1" {
		t.Fatalf("url = %q", link.URL)
	}
	if link.Headers["Authorization"] != "Bearer shortlived" {
		t.Fatalf("Authorization header = %q", link.Headers["Authorization"])
	}
	if link.FileName != "9e9bc2ff-1.csv.gz" {
		t.Fatalf("fileName = %q", link.FileName)
	}

	if _, err := firstDownloadLink(json.RawMessage(`[]`)); err == nil {
		t.Fatal("expected error for empty download array, got nil")
	}

	if _, err := firstDownloadLink(json.RawMessage(`{"downloadUrl":"https://example.com"}`)); err == nil {
		t.Fatal("expected error for object-shaped (non-array) download response, got nil")
	}
}

// gz builds a gzipped CSV part.
func gz(t *testing.T, csv string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write([]byte(csv)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// exportFixture serves a fake ot-insights API: a descriptor with the given
// parts (each with its own token), the parts themselves, and the job status.
type exportFixture struct {
	srv         *httptest.Server
	c           *client.Client
	recordCount int
	badAuth     int // parts fetched with a wrong Authorization
	tenantLeak  int // parts fetched with the tenant bearer
}

func newExportFixture(t *testing.T, parts [][]byte, recordCount int) *exportFixture {
	t.Helper()
	f := &exportFixture{recordCount: recordCount}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	mux.HandleFunc("/api/ot-insights/v1/bulk-export-jobs/run1/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tenant-token" {
			w.WriteHeader(401)
			return
		}
		var links []map[string]any
		for i := range parts {
			links = append(links, map[string]any{
				"id": fmt.Sprint(i), "fileName": fmt.Sprintf("f%d.csv.gz", i),
				"url":     fmt.Sprintf("%s/part/%d", f.srv.URL, i),
				"headers": map[string]string{"Authorization": fmt.Sprintf("part-token-%d", i)},
			})
		}
		_ = json.NewEncoder(w).Encode(links)
	})
	mux.HandleFunc("/part/", func(w http.ResponseWriter, r *http.Request) {
		var i int
		fmt.Sscanf(r.URL.Path, "/part/%d", &i)
		got := r.Header.Get("Authorization")
		if got == "Bearer tenant-token" {
			f.tenantLeak++
		}
		if got != fmt.Sprintf("part-token-%d", i) {
			f.badAuth++
			w.WriteHeader(403)
			return
		}
		_, _ = w.Write(parts[i])
	})
	mux.HandleFunc("/api/ot-insights/v1/bulk-export-jobs/run1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "Completed", "recordCount": f.recordCount})
	})
	f.c = client.New(f.srv.URL, client.NewStaticTokenProvider(f.srv.URL, "tenant-token", false), false)
	return f
}

func threeParts(t *testing.T) [][]byte {
	return [][]byte{
		gz(t, "id,ua\n1,a\n2,\"b\nc\"\n"), // embedded newline: 2 rows
		gz(t, "id,ua\n3,d\n"),
		gz(t, "id,ua\n4,e\n5,f\n6,g\n"),
	}
}

func fastRetries(t *testing.T) {
	old := partRetryBase
	partRetryBase = time.Millisecond
	t.Cleanup(func() { partRetryBase = old })
}

func TestDownloadMergeAllParts(t *testing.T) {
	f := newExportFixture(t, threeParts(t), 6)
	out := filepath.Join(t.TempDir(), "all.csv")
	sum, err := downloadExportJob(f.c, "run1", exportDownloadOpts{Merge: true, Out: out, Verify: true, Concurrency: 2})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if sum.Parts != 3 || sum.Rows != 6 {
		t.Fatalf("summary = %+v", sum)
	}
	if f.badAuth != 0 || f.tenantLeak != 0 {
		t.Fatalf("badAuth=%d tenantLeak=%d", f.badAuth, f.tenantLeak)
	}
	data, _ := os.ReadFile(out)
	want := "id,ua\n1,a\n2,\"b\nc\"\n3,d\n4,e\n5,f\n6,g\n"
	if string(data) != want {
		t.Fatalf("merged = %q, want %q", data, want)
	}
}

func TestDownloadOutDirWritesAllPartsAsIs(t *testing.T) {
	parts := threeParts(t)
	f := newExportFixture(t, parts, 6)
	dir := filepath.Join(t.TempDir(), "parts")
	if _, err := downloadExportJob(f.c, "run1", exportDownloadOpts{OutDir: dir, Verify: true}); err != nil {
		t.Fatalf("download: %v", err)
	}
	for i, p := range parts {
		got, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("part-%04d-f%d.csv.gz", i+1, i)))
		if err != nil || !bytes.Equal(got, p) {
			t.Fatalf("part %d: err=%v equal=%v", i, err, bytes.Equal(got, p))
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("dir has %d entries (leftover tmp?)", len(entries))
	}
}

func TestDownloadMergeGzipOutput(t *testing.T) {
	f := newExportFixture(t, threeParts(t), 6)
	out := filepath.Join(t.TempDir(), "all.csv.gz")
	if _, err := downloadExportJob(f.c, "run1", exportDownloadOpts{Merge: true, Out: out, Verify: true}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(out)
	if _, err := inspectPart(raw); err != nil {
		t.Fatalf("output not gzip csv: %v", err)
	}
	if p, _ := inspectPart(raw); p.rows != 6 {
		t.Fatalf("rows = %d", p.rows)
	}
}

func TestDownloadMergeHeaderMismatch(t *testing.T) {
	parts := [][]byte{gz(t, "id,ua\n1,a\n"), gz(t, "id,other\n2,b\n")}
	f := newExportFixture(t, parts, 2)
	out := filepath.Join(t.TempDir(), "x.csv")
	_, err := downloadExportJob(f.c, "run1", exportDownloadOpts{Merge: true, Out: out, Verify: true})
	if err == nil || !strings.Contains(err.Error(), "different CSV header") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("output file must not exist after header mismatch")
	}
}

func TestDownloadRowCountVerification(t *testing.T) {
	f := newExportFixture(t, threeParts(t), 7) // job claims 7, parts hold 6
	out := filepath.Join(t.TempDir(), "x.csv")
	_, err := downloadExportJob(f.c, "run1", exportDownloadOpts{Merge: true, Out: out, Verify: true})
	apiErr, ok := err.(*client.APIError)
	if !ok || apiErr.Kind != "download_incomplete" || !strings.Contains(apiErr.Detail, "recordCount=7") {
		t.Fatalf("err = %#v", err)
	}
	// --no-verify skips the check
	if _, err := downloadExportJob(f.c, "run1", exportDownloadOpts{Merge: true, Out: out, Verify: false}); err != nil {
		t.Fatalf("no-verify: %v", err)
	}
}

func TestDownloadMergeRequiresOut(t *testing.T) {
	f := newExportFixture(t, threeParts(t), 6)
	if _, err := downloadExportJob(f.c, "run1", exportDownloadOpts{Merge: true}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadPartRetriesTransientFailure(t *testing.T) {
	fastRetries(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write(gz(t, "id\n1\n"))
	}))
	defer srv.Close()
	p, err := fetchPartWithRetry(exportDownloadLink{URL: srv.URL})
	if err != nil || p.rows != 1 || calls != 3 {
		t.Fatalf("err=%v rows=%d calls=%d", err, p.rows, calls)
	}
}

func TestDownloadPartDoesNotRetryClientError(t *testing.T) {
	fastRetries(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(403)
	}))
	defer srv.Close()
	if _, err := fetchPartWithRetry(exportDownloadLink{URL: srv.URL}); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestDownloadPartRetriesTruncatedBody(t *testing.T) {
	fastRetries(t)
	good := gz(t, "id\n1\n2\n")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write(good[:len(good)-6])
			return
		}
		_, _ = w.Write(good)
	}))
	defer srv.Close()
	p, err := fetchPartWithRetry(exportDownloadLink{URL: srv.URL})
	if err != nil || p.rows != 2 || calls != 2 {
		t.Fatalf("err=%v rows=%d calls=%d", err, p.rows, calls)
	}
}

func TestDescriptorFailureIsDownloadListFailed(t *testing.T) {
	old := descriptorRetries
	descriptorRetries = 0
	defer func() { descriptorRetries = old }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// close the connection without a response, like OneTrust does
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()
	c := client.New(srv.URL, client.NewStaticTokenProvider(srv.URL, "t", false), false)
	_, err := downloadExportJob(c, "run1", exportDownloadOpts{OutDir: t.TempDir()})
	apiErr, ok := err.(*client.APIError)
	if !ok || apiErr.Kind != "download_list_failed" || !strings.Contains(apiErr.Hint, "shorter jobs") {
		t.Fatalf("err = %#v", err)
	}
}

func TestOneTrustErrorCodeSurfaced(t *testing.T) {
	cases := []struct {
		status   int
		body     string
		wantCode string
		wantKind string
		wantHint string
	}{
		{400, `{"errors":[{"code":"ERROR_OT_INSIGHTS_COMMAND_MAX_BULK_EXPORT_CONCURRENT_RUNS_PER_TENANT","detail":"Operation Failed","title":"Operation Failed"}],"traceId":"abc123"}`,
			"ERROR_OT_INSIGHTS_COMMAND_MAX_BULK_EXPORT_CONCURRENT_RUNS_PER_TENANT", "validation", "one running bulk-export job per export type"},
		{401, `{"errors":[{"code":"ERROR_OT_INSIGHTS_COMMAND_FEATURE_NOT_ENABLED","detail":"Operation Failed"}],"traceId":"t2"}`,
			"ERROR_OT_INSIGHTS_COMMAND_FEATURE_NOT_ENABLED", "feature_not_enabled", "not a credentials problem"},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := client.New(srv.URL, client.NewStaticTokenProvider(srv.URL, "t", false), false)
		_, err := c.Get(context.Background(), "/x", nil)
		srv.Close()
		apiErr, ok := err.(*client.APIError)
		if !ok || apiErr.Code != tc.wantCode || apiErr.Kind != tc.wantKind || apiErr.TraceID == "" ||
			!strings.Contains(apiErr.Hint, tc.wantHint) || apiErr.Detail != "Operation Failed" {
			t.Fatalf("status %d: err = %#v", tc.status, err)
		}
	}
}

func TestValidateAnonymousReceiptsWindow(t *testing.T) {
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	from := func(days int) string { return yesterday.AddDate(0, 0, -days).Format("2006-01-02") }
	to := yesterday.Format("2006-01-02")
	if _, _, err := validateExportWindowFor("ANONYMOUS_RECEIPTS", from(30), to); err != nil {
		t.Fatalf("30 days should pass: %v", err)
	}
	_, _, err := validateExportWindowFor("ANONYMOUS_RECEIPTS", from(31), to)
	if err == nil || !strings.Contains(err.Error(), "30-day") {
		t.Fatalf("31 days: err = %v", err)
	}
	if _, _, err := validateExportWindowFor("CONSENT_RECEIPTS", from(31), to); err != nil {
		t.Fatalf("CONSENT 31 days should pass: %v", err)
	}
}
