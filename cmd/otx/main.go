package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/nicolasacchi/otx/internal/client"
	"github.com/nicolasacchi/otx/internal/commands"
	"github.com/nicolasacchi/otx/internal/output"
)

var version = "dev"

// resolveVersion returns the ldflags-injected version, or, for `go install
// ...@<tag>` builds where no ldflags are set, the module version recorded in
// the binary's build info.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	commands.SetVersion(resolveVersion())
	err := commands.Execute()
	if err == nil {
		return
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		output.PrintErrorFull(apiErr.Detail, apiErr.Kind, apiErr.Status, apiErr.Hint, apiErr.Code, apiErr.TraceID)
		fmt.Fprintln(os.Stderr, "Error:", apiErr.Error())
		if apiErr.Code != "" || apiErr.TraceID != "" {
			fmt.Fprintf(os.Stderr, "Code: %s  traceId: %s\n", apiErr.Code, apiErr.TraceID)
		}
		if apiErr.Hint != "" {
			fmt.Fprintln(os.Stderr, "Hint:", apiErr.Hint)
		}
		os.Exit(apiErr.ExitCode())
	}
	output.PrintError(err.Error(), "unexpected", 0, "")
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
