package updatecheck

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type NotifyOptions struct {
	CurrentVersion string
	Args           []string
	HomeDir        string
	Endpoint       string
	Stderr         *os.File
	Now            func() time.Time
}

func MaybeNotify(ctx context.Context, opts NotifyOptions) {
	if !ShouldCheck(opts.CurrentVersion, opts.Args, opts.Stderr, os.Getenv) {
		return
	}
	result, err := Check(ctx, Options{CurrentVersion: opts.CurrentVersion, HomeDir: opts.HomeDir, Endpoint: opts.Endpoint, Now: opts.Now})
	if err != nil || !result.UpdateAvailable {
		return
	}
	WriteNotice(opts.Stderr, opts.CurrentVersion, result)
}

func WriteNotice(w io.Writer, currentVersion string, result Result) {
	url := result.URL
	if url == "" {
		url = "https://github.com/jmeiracorbal/mnemo/releases/latest"
	}
	_, _ = fmt.Fprintf(w, "[mnemo] update available: v%s is installed, v%s is available.\n", Normalize(currentVersion), result.LatestVersion)
	_, _ = fmt.Fprintln(w, "[mnemo] run 'mnemo update' to review, confirm, download, and install the update.")
	_, _ = fmt.Fprintf(w, "[mnemo] release notes: %s\n", url)
}

func ShouldPrompt(currentVersion string, args []string, stdin *os.File, stderr *os.File, getenv func(string) string) bool {
	if !ShouldCheck(currentVersion, args, stderr, getenv) {
		return false
	}
	if stdin == nil || !isTerminal(stdin) {
		return false
	}
	return true
}

func ShouldCheck(currentVersion string, args []string, stderr *os.File, getenv func(string) string) bool {
	if Normalize(currentVersion) == "" || Normalize(currentVersion) == "dev" {
		return false
	}
	if getenv("MNEMO_NO_UPDATE_CHECK") != "" || getenv("MNEMO_DISABLE_UPDATE_CHECK") != "" {
		return false
	}
	if getenv("MNEMO_SOURCE") == "hook" || getenv("MNEMO_SOURCE") == "mcp" || getenv("MNEMO_MCP_CLIENT") != "" {
		return false
	}
	if len(args) == 0 {
		return false
	}
	if commandSkipsUpdateCheck(args) {
		return false
	}
	if stderr == nil || !isTerminal(stderr) {
		return false
	}
	return true
}

func commandSkipsUpdateCheck(args []string) bool {
	command := args[0]
	switch command {
	case "mcp", "json", "json-merge", "extract-transcript", "update", "--version", "version":
		return true
	}
	for _, arg := range args[1:] {
		if arg == "--json" || strings.HasPrefix(arg, "--json=") {
			return true
		}
	}
	return false
}

func isTerminal(file *os.File) bool {
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

var _ io.Writer = (*os.File)(nil)
