package commands

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nicolasacchi/otx/internal/async"
	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

// exportJobsPath is the real bulk-export microservice route. OneTrust's docs
// (and several third-party writeups) reference /api/bulk-export/v1/exports,
// but that path does not exist on this tenant's gateway — it 200s with the
// Angular SPA shell (see client.isNonJSONBody), byte-identical to a totally
// fabricated path. The live service is ot-insights.
const exportJobsPath = "/api/ot-insights/v1/bulk-export-jobs"

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Bulk exports via ot-insights (CONSENT_RECEIPTS, ANONYMOUS_RECEIPTS = cookie-banner receipts, COOKIE_RECEIPTS, DATA_SUBJECTS)",
}

// -------- bulk --------

var exportBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Bulk export jobs (ot-insights, 10-day retention)",
}

var (
	exportBulkType     string
	exportBulkName     string
	exportBulkFormat   string
	exportBulkFrom     string
	exportBulkTo       string
	exportBulkAwait    bool
	exportBulkInterval time.Duration
	exportBulkTimeout  time.Duration
	exportBulkOut      string
	exportBulkOutDir   string
	exportBulkMerge    bool
	exportBulkNoVerify bool
	exportBulkConc     int
)

var exportBulkCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a bulk-export job",
	Long: `Create a bulk-export job on the ot-insights service.

Constraints enforced client-side before the request is sent (all documented
by OneTrust, not guessed):
  - --to minus --from must be <= 180 days (ANONYMOUS_RECEIPTS: <= 30 days)
  - --to must be yesterday (UTC) or earlier — never today or the future
  - exports are capped at ~1M records / ~8MB
  - accounts get 20 exports/month; the response's availableCredit field
    reports what is left
  - completed jobs expire 10 days after creation

The cookie-banner (anonymous) receipts are ANONYMOUS_RECEIPTS; on tenants
where they live there, COOKIE_RECEIPTS returns 0 rows. Only one job per
export type can run at a time.

With --await, --out FILE downloads all parts merged into one CSV and
--out-dir DIR keeps the raw parts (same semantics as 'export bulk download';
the row count is verified against recordCount).

COOKIE_RECEIPTS is under OneTrust Public Preview and returns 401
ERROR_OT_INSIGHTS_COMMAND_FEATURE_NOT_ENABLED until OneTrust enables it for
this tenant — that is a real per-tenant gate, not a bug in otx.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate the user's own input BEFORE the --confirm/credential guard, so
		// a bad --type or date window reports the actual problem instead of being
		// masked by write_locked. Neither check touches the network.
		if exportBulkType == "" {
			return validationErr("--type is required (CONSENT_RECEIPTS / ANONYMOUS_RECEIPTS / COOKIE_RECEIPTS / DATA_SUBJECTS / DATA_SUBJECTS_BULK_ACTIONS / DATA_SUBJECTS_BULK_ADD_PURPOSE)")
		}
		start, end, err := validateExportWindowFor(exportBulkType, exportBulkFrom, exportBulkTo)
		if err != nil {
			return err
		}
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		name := exportBulkName
		if name == "" {
			name = fmt.Sprintf("otx-%s-%s", strings.ToLower(exportBulkType), time.Now().UTC().Format("20060102-150405"))
		}
		format := exportBulkFormat
		if format == "" {
			format = "CSV"
		}
		body := map[string]any{
			"exportType":   exportBulkType,
			"name":         name,
			"startDate":    start.Format("2006-01-02"),
			"endDate":      end.Format("2006-01-02"),
			"exportFormat": format,
		}
		resp, err := c.Post(context.Background(), exportJobsPath, body)
		if err != nil {
			return err
		}
		// The create success shape is unverified against this tenant (our only
		// live observation was a 401 feature-gate error for COOKIE_RECEIPTS).
		// Extract the job id defensively rather than assume a field name.
		jobID := extractExportJobID(resp)
		if !exportBulkAwait {
			return printJSONValue(json.RawMessage(resp))
		}
		if jobID == "" {
			return validationErr("create succeeded but the response has no runId/id/exportId to poll — cannot --await (response: %s)", string(resp))
		}
		// Poll for completion, optionally download.
		final, err := async.Wait(context.Background(), c,
			exportJobsPath+"/"+url.PathEscape(jobID),
			async.Options{Interval: exportBulkInterval, Timeout: exportBulkTimeout})
		if err != nil {
			return err
		}
		printExportSummary(final)
		if exportBulkOut != "" || exportBulkOutDir != "" {
			if err := runDownload(c, jobID, exportBulkOut != "", exportBulkOut); err != nil {
				return err
			}
		}
		return printJSONValue(json.RawMessage(final))
	},
}

var exportBulkStatusCmd = &cobra.Command{
	Use:   "status <export-id>",
	Short: "Check status of a bulk-export job (runId from 'create' or 'list')",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), exportJobsPath+"/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		printExportSummary(body)
		return printJSONValue(json.RawMessage(body))
	},
}

var exportBulkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent bulk-export jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := c.Get(context.Background(), exportJobsPath, nil)
		if err != nil {
			return err
		}
		return printData("export.bulk.list", flattenItems(body))
	},
}

var exportBulkDownloadCmd = &cobra.Command{
	Use:   "download <export-id>",
	Short: "Download ALL parts of a finished export (per-part short-lived tokens)",
	Long: `Download a finished export.

GET .../{runId}/download returns a JSON array of PARTS (a 1-day job had 167,
a 29-day job 1,045); each part is a gzipped CSV with its own header row and
its own url + short-lived Authorization token (NOT your tenant token). otx
fetches every part (6 in parallel, per-part retries) and then checks that the
total data rows equal the job's recordCount (skip with --no-verify).

Output modes:
  --out-dir DIR        write each part's .csv.gz as-is (default ./<runId>/),
                       files named part-NNNN-<fileName> in descriptor order
  --merge --out FILE   one decompressed CSV with a single header, parts in
                       descriptor order; FILE '-' = stdout; a FILE ending
                       .gz is gzipped. Errors if part headers differ.

For very large jobs OneTrust often times out while LISTING the files
(error kind download_list_failed): create shorter jobs instead.

Receipt rows are personal data — mind where you write them.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		if exportBulkOut != "" && !exportBulkMerge {
			return validationErr("--out needs --merge (one decompressed CSV); to keep the parts as-is use --out-dir DIR")
		}
		return runDownload(c, args[0], exportBulkMerge, exportBulkOut)
	},
}

// runDownload runs downloadExportJob with the shared flags and prints the
// summary. merge/out select the single-CSV mode; otherwise parts go to
// --out-dir (default ./<runId>/).
func runDownload(c *client.Client, runID string, merge bool, out string) error {
	sum, err := downloadExportJob(c, runID, exportDownloadOpts{
		OutDir:      exportBulkOutDir,
		Merge:       merge,
		Out:         out,
		Verify:      !exportBulkNoVerify,
		Concurrency: exportBulkConc,
	})
	if err != nil {
		return err
	}
	printDownloadSummary(sum)
	if merge && out != "-" {
		fmt.Fprintf(os.Stderr, "→ wrote %s\n", out)
	} else if !merge {
		dir := exportBulkOutDir
		if dir == "" {
			dir = runID
		}
		fmt.Fprintf(os.Stderr, "→ wrote parts to %s/\n", dir)
	}
	return nil
}

// -------- poll --------

var exportPollCmd = &cobra.Command{
	Use:   "poll <export-id>",
	Short: "Block until an export job finishes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		body, err := async.Wait(context.Background(), c,
			exportJobsPath+"/"+url.PathEscape(args[0]),
			async.Options{Interval: exportBulkInterval, Timeout: exportBulkTimeout})
		if err != nil {
			return err
		}
		printExportSummary(body)
		return printJSONValue(json.RawMessage(body))
	},
}

// validateExportWindow parses and validates --from/--to against the
// documented ot-insights constraints: both required, --from <= --to,
// --to no later than yesterday (UTC), and a span of at most 180 days.
func validateExportWindow(fromStr, toStr string) (start, end time.Time, err error) {
	return validateExportWindowFor("", fromStr, toStr)
}

// maxWindowDays is the documented per-job span limit for an export type.
func maxWindowDays(exportType string) int {
	if strings.EqualFold(exportType, "ANONYMOUS_RECEIPTS") {
		return 30
	}
	return 180
}

// validateExportWindowFor is validateExportWindow with the per-type span
// limit (ANONYMOUS_RECEIPTS: 30 days, everything else 180).
func validateExportWindowFor(exportType, fromStr, toStr string) (start, end time.Time, err error) {
	start, err = parseExportDate("from", fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err = parseExportDate("to", toStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, validationErr("--to (%s) is before --from (%s)", toStr, fromStr)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if !end.Before(today) {
		return time.Time{}, time.Time{}, validationErr(
			"--to (%s) must be yesterday (%s UTC) or earlier — OneTrust bulk-export never accepts today or a future end date",
			toStr, today.AddDate(0, 0, -1).Format("2006-01-02"))
	}
	maxDays := maxWindowDays(exportType)
	if span := end.Sub(start); span > time.Duration(maxDays)*24*time.Hour {
		return time.Time{}, time.Time{}, validationErr(
			"date window --from %s to --to %s spans %d days, exceeding the %d-day maximum%s",
			fromStr, toStr, int(span.Hours()/24), maxDays, map[bool]string{true: " for ANONYMOUS_RECEIPTS", false: ""}[maxDays == 30])
	}
	return start, end, nil
}

// parseExportDate parses a required yyyy-mm-dd flag value.
func parseExportDate(flag, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, validationErr("--%s is required (yyyy-mm-dd)", flag)
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, validationErr("--%s must be yyyy-mm-dd (got %q)", flag, value)
	}
	return t, nil
}

// extractExportJobID pulls the job id out of a bulk-export-jobs response
// defensively: the observed list/status shape uses "runId", but the create
// success shape is unverified (our tenant 401s before returning one), so we
// also accept the more common "id"/"exportId" fallbacks rather than assume.
func extractExportJobID(resp json.RawMessage) string {
	for _, key := range []string{"runId", "id", "exportId"} {
		if v := gjson.GetBytes(resp, key).String(); v != "" {
			return v
		}
	}
	return ""
}

// printExportSummary writes a one-line human status to stderr, and calls
// out a completed-but-empty export explicitly: recordCount:0 is a real,
// confusing outcome (an empty download) that otherwise passes silently.
func printExportSummary(body json.RawMessage) {
	status := gjson.GetBytes(body, "status")
	if !status.Exists() {
		return
	}
	line := fmt.Sprintf("→ status=%s", status.String())
	recordCount := gjson.GetBytes(body, "recordCount")
	if recordCount.Exists() {
		line += fmt.Sprintf(" recordCount=%d", recordCount.Int())
	}
	if availableCredit := gjson.GetBytes(body, "availableCredit"); availableCredit.Exists() {
		line += fmt.Sprintf(" availableCredit=%d", availableCredit.Int())
	}
	fmt.Fprintln(os.Stderr, line)
	if strings.EqualFold(status.String(), "Completed") && recordCount.Exists() && recordCount.Int() == 0 {
		fmt.Fprintln(os.Stderr, "⚠ export completed with recordCount=0 — no records matched this window/filter; the downloaded file will be empty")
	}
}

// exportDownloadLink is one element of the JSON array returned by
// GET {exportJobsPath}/{runId}/download. There is no "downloadUrl" field
// anywhere in this API. A job is split into MANY parts (a 1-day job had 167,
// a 29-day job 1,045); each part has its own url and its own ~1-hour
// headers.Authorization token, distinct from the caller's tenant bearer.
// Every part is a gzipped CSV with its own header row.
type exportDownloadLink struct {
	ID       string            `json:"id"`
	Headers  map[string]string `json:"headers"`
	URL      string            `json:"url"`
	FileName string            `json:"fileName"`
}

// parseExportDownloadLinks decodes the array-shaped download response.
func parseExportDownloadLinks(body json.RawMessage) ([]exportDownloadLink, error) {
	var links []exportDownloadLink
	if err := json.Unmarshal(body, &links); err != nil {
		return nil, &client.APIError{Kind: "api_error", Detail: fmt.Sprintf("unexpected download response shape (expected a JSON array): %v", err)}
	}
	return links, nil
}

// firstDownloadLink returns the first element of the download array, or a
// clear error if the job has no file yet. Kept for callers that only need to
// know a file exists; downloads use every element (see downloadExportJob).
func firstDownloadLink(body json.RawMessage) (exportDownloadLink, error) {
	links, err := parseExportDownloadLinks(body)
	if err != nil {
		return exportDownloadLink{}, err
	}
	if len(links) == 0 {
		return exportDownloadLink{}, validationErr("export download endpoint returned no files — the job may not be complete yet, or its 10-day retention window has expired")
	}
	return links[0], nil
}

// Tunables, vars so tests can shrink them.
var (
	descriptorRetries  = 8
	partAttempts       = 5
	partRetryBase      = 500 * time.Millisecond
	downloadConcurrent = 6
	partHTTPClient     = &http.Client{Timeout: 3 * time.Minute}
)

// exportDownloadOpts selects the output mode of downloadExportJob.
type exportDownloadOpts struct {
	OutDir      string // write each part's .csv.gz as-is into this dir
	Merge       bool   // write one decompressed CSV with a single header
	Out         string // merge target; "-" = stdout; *.gz = gzip the merged CSV
	Verify      bool   // compare data rows to the job's recordCount
	Concurrency int
}

type exportDownloadSummary struct {
	Parts int
	Rows  int64
	Bytes int64
}

type fetchedPart struct {
	raw    []byte
	header []string
	rows   int64
}

// fetchDownloadLinks fetches the descriptor array. The endpoint is flaky for
// large jobs (the server drops the connection after ~10s with no response),
// so it gets many more retries than the default.
func fetchDownloadLinks(c *client.Client, runID string) ([]exportDownloadLink, error) {
	body, err := c.WithMaxRetries(descriptorRetries).Get(context.Background(), exportJobsPath+"/"+url.PathEscape(runID)+"/download", nil)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			return nil, err
		}
		return nil, &client.APIError{
			Kind:   "download_list_failed",
			Detail: fmt.Sprintf("could not list the export's files after %d retries: %v", descriptorRetries, err),
			Hint:   "OneTrust times out when listing the files of large jobs (~1,000 parts); create shorter jobs (e.g. a few days per job) and download each, or retry shortly after the job completes",
		}
	}
	links, err := parseExportDownloadLinks(body)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, validationErr("export download endpoint returned no files — the job may not be complete yet, or its 10-day retention window has expired")
	}
	return links, nil
}

// downloadExportJob downloads EVERY part of the job, with bounded concurrency
// and per-part retries, then writes them per opts and (optionally) verifies
// the total data-row count against the job's recordCount.
func downloadExportJob(c *client.Client, runID string, opts exportDownloadOpts) (exportDownloadSummary, error) {
	var sum exportDownloadSummary
	if opts.Merge && opts.Out == "" {
		return sum, validationErr("--merge requires --out FILE (use '-' for stdout)")
	}
	if !opts.Merge && opts.OutDir == "" {
		opts.OutDir = runID
	}
	links, err := fetchDownloadLinks(c, runID)
	if err != nil {
		return sum, err
	}
	for i, l := range links {
		if l.URL == "" {
			return sum, validationErr("export download part %d has no url", i)
		}
	}
	parts, err := fetchAllParts(links, opts.Concurrency)
	if err != nil {
		return sum, err
	}
	sum.Parts = len(parts)
	for _, p := range parts {
		sum.Rows += p.rows
		sum.Bytes += int64(len(p.raw))
	}
	if opts.Merge {
		err = writeMerged(parts, opts.Out)
	} else {
		err = writeParts(parts, links, opts.OutDir)
	}
	if err != nil {
		return sum, err
	}
	if opts.Verify {
		if err := verifyRecordCount(c, runID, sum.Rows); err != nil {
			return sum, err
		}
	}
	return sum, nil
}

// fetchAllParts downloads parts concurrently, preserving descriptor order.
func fetchAllParts(links []exportDownloadLink, concurrency int) ([]fetchedPart, error) {
	if concurrency <= 0 {
		concurrency = downloadConcurrent
	}
	parts := make([]fetchedPart, len(links))
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, concurrency)
	)
	for i := range links {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			p, err := fetchPartWithRetry(links[i])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("part %d/%d (%s): %w", i+1, len(links), links[i].FileName, err)
				}
				return
			}
			parts[i] = p
		}(i)
	}
	wg.Wait()
	if firstErr != nil {
		var apiErr *client.APIError
		if errors.As(firstErr, &apiErr) {
			return nil, firstErr
		}
		return nil, &client.APIError{Kind: "download_failed", Detail: firstErr.Error(), Hint: "re-run the download; part tokens last ~1 hour, so re-listing is needed after that"}
	}
	return parts, nil
}

// fetchPartWithRetry fetches one part with its own Authorization header and
// validates that it is a well-formed gzipped CSV (so a truncated body is
// retried instead of silently accepted).
func fetchPartWithRetry(l exportDownloadLink) (fetchedPart, error) {
	var lastErr error
	for attempt := 0; attempt < partAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(partRetryBase << (attempt - 1))
		}
		raw, retry, err := fetchPresigned(l.URL, l.Headers)
		if err == nil {
			var p fetchedPart
			p, err = inspectPart(raw)
			if err == nil {
				return p, nil
			}
			retry = true
		}
		lastErr = err
		if !retry {
			return fetchedPart{}, err
		}
	}
	return fetchedPart{}, fmt.Errorf("giving up after %d attempts: %w", partAttempts, lastErr)
}

// inspectPart gunzips raw and returns its header and data-row count. Rows are
// counted as CSV records (not lines), so embedded newlines don't skew it.
func inspectPart(raw []byte) (fetchedPart, error) {
	p := fetchedPart{raw: raw}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return p, fmt.Errorf("not gzip: %w", err)
	}
	r := csv.NewReader(zr)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = true
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return p, fmt.Errorf("bad csv/gzip body: %w", err)
		}
		if first {
			p.header = append([]string(nil), rec...)
			first = false
			continue
		}
		p.rows++
	}
	return p, nil
}

// fetchPresigned GETs urlStr with exactly the given headers (never the tenant
// bearer) and returns the body. retry reports whether the failure is
// transient (network error, 429, 5xx).
func fetchPresigned(urlStr string, headers map[string]string) (body []byte, retry bool, err error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, false, err
	}
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := partHTTPClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, resp.StatusCode == 429 || resp.StatusCode >= 500,
			&client.APIError{Status: resp.StatusCode, Kind: "api_error", Detail: "presigned download failed"}
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return body, false, nil
}

// writeParts writes each part's gzip as-is, atomically (tmp + rename). File
// names are prefixed with the descriptor index so order is recoverable.
func writeParts(parts []fetchedPart, links []exportDownloadLink, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, p := range parts {
		name := filepath.Base(links[i].FileName)
		if name == "" || name == "." || name == "/" {
			name = "part.csv.gz"
		}
		path := filepath.Join(dir, fmt.Sprintf("part-%04d-%s", i+1, name))
		if err := writeFileAtomic(path, p.raw); err != nil {
			return err
		}
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// writeMerged writes one decompressed CSV with a single header, in
// descriptor order. All parts must share the same header.
func writeMerged(parts []fetchedPart, out string) error {
	for i := 1; i < len(parts); i++ {
		if !equalStrings(parts[0].header, parts[i].header) {
			return &client.APIError{Kind: "download_failed",
				Detail: fmt.Sprintf("part %d has a different CSV header than part 1 — refusing to merge", i+1),
				Hint:   "download with --out-dir to keep the parts separate"}
		}
	}
	var (
		dst   io.Writer = os.Stdout
		file  *os.File
		final string
	)
	if out != "-" {
		final = out
		var err error
		file, err = os.OpenFile(out+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		dst = file
	}
	var gzw *gzip.Writer
	if final != "" && strings.HasSuffix(out, ".gz") {
		gzw = gzip.NewWriter(dst)
		dst = gzw
	}
	bw := bufio.NewWriter(dst)
	cw := csv.NewWriter(bw)
	err := func() error {
		for i, p := range parts {
			zr, err := gzip.NewReader(bytes.NewReader(p.raw))
			if err != nil {
				return err
			}
			r := csv.NewReader(zr)
			r.FieldsPerRecord = -1
			r.LazyQuotes = true
			r.ReuseRecord = true
			first := true
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if first {
					first = false
					if i > 0 {
						continue // single header: skip repeats
					}
				}
				if err := cw.Write(rec); err != nil {
					return err
				}
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return err
		}
		return bw.Flush()
	}()
	if gzw != nil && err == nil {
		err = gzw.Close()
	}
	if file != nil {
		if cerr := file.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(out + ".tmp")
			return err
		}
		return os.Rename(out+".tmp", final)
	}
	return err
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// verifyRecordCount compares the downloaded data rows to the job's
// recordCount. If the status carries no recordCount it only warns.
func verifyRecordCount(c *client.Client, runID string, rows int64) error {
	body, err := c.Get(context.Background(), exportJobsPath+"/"+url.PathEscape(runID), nil)
	if err != nil {
		return err
	}
	rc := gjson.GetBytes(body, "recordCount")
	if !rc.Exists() {
		fmt.Fprintln(os.Stderr, "⚠ job status has no recordCount — row count not verified")
		return nil
	}
	if rc.Int() != rows {
		return &client.APIError{Kind: "download_incomplete",
			Detail: fmt.Sprintf("downloaded %d data rows but the job reports recordCount=%d", rows, rc.Int()),
			Hint:   "the download is incomplete or the job changed; re-run it (use --no-verify to skip this check)"}
	}
	return nil
}

func printDownloadSummary(s exportDownloadSummary) {
	fmt.Fprintf(os.Stderr, "→ downloaded %d parts, %d rows, %d bytes (gzipped)\n", s.Parts, s.Rows, s.Bytes)
}

func init() {
	exportBulkCreateCmd.Flags().StringVar(&exportBulkType, "type", "", "Export type: CONSENT_RECEIPTS / ANONYMOUS_RECEIPTS (cookie-banner receipts, max 30 days) / COOKIE_RECEIPTS / DATA_SUBJECTS / DATA_SUBJECTS_BULK_ACTIONS / DATA_SUBJECTS_BULK_ADD_PURPOSE")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkName, "name", "", "Job name (default: otx-<type>-<timestamp>)")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkFormat, "format", "CSV", "Export file format")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkFrom, "from", "", "Start date, yyyy-mm-dd (required)")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkTo, "to", "", "End date, yyyy-mm-dd (required; yesterday UTC at the latest)")
	exportBulkCreateCmd.Flags().BoolVar(&exportBulkAwait, "await", false, "Poll until COMPLETED before returning")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkOut, "out", "", "With --await: download all parts merged into this single CSV ('-' = stdout)")
	exportBulkCreateCmd.Flags().StringVar(&exportBulkOutDir, "out-dir", "", "With --await: download the raw .csv.gz parts into this directory")
	exportBulkDownloadCmd.Flags().StringVar(&exportBulkOut, "out", "", "With --merge: merged CSV path ('-' = stdout)")
	exportBulkDownloadCmd.Flags().StringVar(&exportBulkOutDir, "out-dir", "", "Directory for the raw .csv.gz parts (default ./<export-id>/)")
	exportBulkDownloadCmd.Flags().BoolVar(&exportBulkMerge, "merge", false, "Merge all parts into one decompressed CSV (needs --out)")
	exportBulkDownloadCmd.Flags().BoolVar(&exportBulkNoVerify, "no-verify", false, "Skip the rows == recordCount check")
	exportBulkDownloadCmd.Flags().IntVar(&exportBulkConc, "concurrency", 6, "Parallel part downloads")
	exportPollCmd.Flags().DurationVar(&exportBulkInterval, "interval", 5*time.Second, "Poll cadence")
	exportPollCmd.Flags().DurationVar(&exportBulkTimeout, "timeout", 30*time.Minute, "Hard timeout")
	exportBulkCreateCmd.Flags().DurationVar(&exportBulkInterval, "interval", 5*time.Second, "Poll cadence (with --await)")
	exportBulkCreateCmd.Flags().DurationVar(&exportBulkTimeout, "timeout", 30*time.Minute, "Hard timeout (with --await)")

	exportBulkCmd.AddCommand(exportBulkCreateCmd, exportBulkStatusCmd, exportBulkListCmd, exportBulkDownloadCmd)
	exportCmd.AddCommand(exportBulkCmd, exportPollCmd)
	rootCmd.AddCommand(exportCmd)
}
