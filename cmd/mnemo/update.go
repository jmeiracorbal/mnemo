package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/updatecheck"
)

const installScriptURL = "https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh"

type updateOptions struct {
	AssumeYes bool
	CheckOnly bool
	JSON      bool
	Agent     string
}

type updateStatus struct {
	Checked         bool   `json:"checked"`
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	URL             string `json:"url,omitempty"`
	Installed       bool   `json:"installed"`
	Message         string `json:"message,omitempty"`
}

type updateRuntime struct {
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	install func(context.Context, string, string, io.Writer, io.Writer) error
	check   func(context.Context, updatecheck.Options) (updatecheck.Result, error)
}

func defaultUpdateRuntime() updateRuntime {
	return updateRuntime{
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		install: installMnemoRelease,
		check:   updatecheck.Check,
	}
}

func runUpdate() {
	status, err := runUpdateCommand(context.Background(), os.Args[2:], defaultUpdateRuntime())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo update: %v\n", err)
		os.Exit(1)
	}
	if status.UpdateAvailable && !status.Installed && !status.Checked {
		os.Exit(1)
	}
}

func runUpdateCommand(ctx context.Context, args []string, rt updateRuntime) (updateStatus, error) {
	opts, err := parseUpdateArgs(args)
	if err != nil {
		return updateStatus{}, err
	}
	if opts.JSON && !opts.CheckOnly {
		return updateStatus{}, errors.New("--json requires --check")
	}
	result, err := rt.check(ctx, updatecheck.Options{CurrentVersion: version, Force: true})
	status := statusFromUpdateResult(result)
	if err != nil {
		return status, err
	}
	if opts.JSON {
		return printUpdateJSON(status, rt.stdout)
	}
	if !result.UpdateAvailable {
		printNoUpdate(status, rt.stdout)
		return status, nil
	}
	updatecheck.WriteNotice(rt.stderr, version, result)
	if opts.CheckOnly {
		return status, nil
	}
	if !opts.AssumeYes {
		approved, promptErr := promptForUpdate(rt.stdin, rt.stderr)
		if promptErr != nil {
			return status, promptErr
		}
		if !approved {
			status.Message = "update declined"
			_, _ = fmt.Fprintln(rt.stderr, "[mnemo] update skipped.")
			return status, nil
		}
	}
	if err := rt.install(ctx, result.LatestVersion, opts.Agent, rt.stdout, rt.stderr); err != nil {
		status.Message = err.Error()
		return status, err
	}
	status.Installed = true
	status.Message = "update installed"
	_, _ = fmt.Fprintf(rt.stderr, "[mnemo] update installed: v%s\n", result.LatestVersion)
	return status, nil
}

func parseUpdateArgs(args []string) (updateOptions, error) {
	opts := updateOptions{Agent: "all"}
	for _, arg := range args {
		switch {
		case arg == "--yes" || arg == "-y":
			opts.AssumeYes = true
		case arg == "--check":
			opts.CheckOnly = true
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(strings.TrimPrefix(arg, "--agent="))
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	switch opts.Agent {
	case "auto", "all", "claudecode", "cursor", "windsurf", "codex", "opencode", "fx", "pi":
		return opts, nil
	default:
		return opts, fmt.Errorf("unknown agent %q", opts.Agent)
	}
}

func maybeOfferUpdate(ctx context.Context, _ []string, rt updateRuntime) {
	result, err := rt.check(ctx, updatecheck.Options{CurrentVersion: version})
	if err != nil || !result.UpdateAvailable {
		return
	}
	updatecheck.WriteNotice(rt.stderr, version, result)
	approved, err := promptForUpdate(rt.stdin, rt.stderr)
	if err != nil || !approved {
		if err == nil {
			_, _ = fmt.Fprintln(rt.stderr, "[mnemo] update skipped.")
		}
		return
	}
	if err := rt.install(context.Background(), result.LatestVersion, "all", rt.stdout, rt.stderr); err != nil {
		_, _ = fmt.Fprintf(rt.stderr, "[mnemo] update failed: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(rt.stderr, "[mnemo] update installed: v%s\n", result.LatestVersion)
}

func promptForUpdate(stdin io.Reader, stderr io.Writer) (bool, error) {
	_, _ = fmt.Fprint(stderr, "[mnemo] Update now? [y/N] ")
	answer, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func statusFromUpdateResult(result updatecheck.Result) updateStatus {
	return updateStatus{
		Checked:         result.Checked,
		UpdateAvailable: result.UpdateAvailable,
		CurrentVersion:  result.CurrentVersion,
		LatestVersion:   result.LatestVersion,
		URL:             result.URL,
		Message:         result.Message,
	}
}

func printUpdateJSON(status updateStatus, stdout io.Writer) (updateStatus, error) {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return status, err
	}
	_, _ = fmt.Fprintln(stdout, string(data))
	return status, nil
}

func printNoUpdate(status updateStatus, stdout io.Writer) {
	if status.Message != "" {
		_, _ = fmt.Fprintf(stdout, "mnemo update: %s\n", status.Message)
		return
	}
	_, _ = fmt.Fprintf(stdout, "mnemo update: v%s is already current.\n", status.CurrentVersion)
}

func installMnemoRelease(ctx context.Context, latestVersion string, agent string, stdout io.Writer, stderr io.Writer) error {
	tag := "v" + updatecheck.Normalize(latestVersion)
	if tag == "v" {
		return errors.New("latest version is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, installScriptURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download installer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download installer returned %s", resp.Status)
	}
	tmpDir, err := os.MkdirTemp("", "mnemo-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	scriptPath := filepath.Join(tmpDir, "install.sh")
	script, err := os.OpenFile(scriptPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(script, io.LimitReader(resp.Body, 1024*1024)); err != nil {
		_ = script.Close()
		return err
	}
	if err := script.Close(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "bash", scriptPath, "--agent="+agent)
	cmd.Env = append(os.Environ(), "MNEMO_VERSION="+tag)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
