package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "File attachments (upload to /api/document/v2/, download from /api/document/v3/)",
}

var (
	attachUploadPath   string
	attachDownloadOut  string
	attachSubjectZipID string
)

var attachUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a file (multipart, max 64MB) — returns Id + downloadUrl",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getWriteClient()
		if err != nil {
			return err
		}
		if attachUploadPath == "" {
			return validationErr("--file is required")
		}
		info, err := os.Stat(attachUploadPath)
		if err != nil {
			return err
		}
		if info.Size() > 64*1024*1024 {
			return validationErr("file exceeds 64MB upload limit")
		}
		body, contentType, err := buildMultipart(attachUploadPath)
		if err != nil {
			return err
		}
		resp, err := c.Raw(context.Background(), "POST", "/api/document/v2/attachments", contentType, body)
		if err != nil {
			return err
		}
		return printJSONValue(json.RawMessage(resp))
	},
}

var attachDownloadCmd = &cobra.Command{
	Use:   "download <attachment-id>",
	Short: "Download an attachment to --out (or stdout)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		raw, err := c.Get(context.Background(), "/api/document/v3/attachments/"+url.PathEscape(args[0])+"/content", nil)
		if err != nil {
			return err
		}
		// Some OneTrust deployments return the binary inline; others return a
		// presigned downloadUrl. Handle both.
		dl := gjson.GetBytes(raw, "downloadUrl").String()
		if dl != "" {
			return streamToOutput(c, dl, attachDownloadOut)
		}
		// Inline content fallback — bytes returned as-is.
		if attachDownloadOut == "" || attachDownloadOut == "-" {
			_, err := os.Stdout.Write(raw)
			return err
		}
		return os.WriteFile(attachDownloadOut, raw, 0644)
	},
}

var attachSubjectZipCmd = &cobra.Command{
	Use:   "subject-zip <data-subject-id>",
	Short: "Download all consent attachments for a data subject as a zip",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := getClient()
		if err != nil {
			return err
		}
		raw, err := c.Get(context.Background(), "/api/consent/v2/attachments/"+url.PathEscape(args[0]), nil)
		if err != nil {
			return err
		}
		if attachDownloadOut == "" {
			attachDownloadOut = fmt.Sprintf("subject-%s.zip", args[0])
		}
		return os.WriteFile(attachDownloadOut, raw, 0644)
	},
}

// streamToOutput downloads a presigned URL using a fresh request that bypasses
// the OAuth bearer (presigned URLs carry their own auth).
func streamToOutput(c *client.Client, presigned, outPath string) error {
	if !strings.HasPrefix(presigned, "http") {
		presigned = c.BaseURL() + presigned
	}
	req, err := http.NewRequest(http.MethodGet, presigned, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return &client.APIError{Status: resp.StatusCode, Kind: "api_error", Detail: "presigned download failed"}
	}
	var w io.Writer = os.Stdout
	if outPath != "" && outPath != "-" {
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func init() {
	attachUploadCmd.Flags().StringVar(&attachUploadPath, "file", "", "Path to file to upload")
	attachDownloadCmd.Flags().StringVar(&attachDownloadOut, "out", "", "Output path (default: stdout)")
	attachSubjectZipCmd.Flags().StringVar(&attachDownloadOut, "out", "", "Output zip path (default: subject-<id>.zip)")
	attachSubjectZipCmd.Flags().StringVar(&attachSubjectZipID, "data-subject-id", "", "(alternative to positional arg)")

	attachCmd.AddCommand(attachUploadCmd, attachDownloadCmd, attachSubjectZipCmd)
	rootCmd.AddCommand(attachCmd)
}
