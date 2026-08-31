package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/updatecheck"
)

func TestRunUpdateCommandCheckJSONDoesNotInstall(t *testing.T) {
	restore := setTestVersion("v0.33.1")
	defer restore()
	installed := false
	var stdout, stderr bytes.Buffer
	status, err := runUpdateCommand(context.Background(), []string{"--check", "--json"}, updateRuntime{
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &stderr,
		check:  fakeUpdateCheck("0.34.0"),
		install: func(context.Context, string, string, io.Writer, io.Writer) error {
			installed = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run update: %v", err)
	}
	if installed {
		t.Fatal("check-only JSON path installed update")
	}
	if !status.Checked || !status.UpdateAvailable || status.LatestVersion != "0.34.0" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if !strings.Contains(stdout.String(), `"update_available": true`) {
		t.Fatalf("missing JSON status: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON check wrote stderr: %q", stderr.String())
	}
}

func TestRunUpdateCommandPromptsAndInstalls(t *testing.T) {
	restore := setTestVersion("v0.33.1")
	defer restore()
	var stdout, stderr bytes.Buffer
	var gotVersion, gotAgent string
	status, err := runUpdateCommand(context.Background(), nil, updateRuntime{
		stdin:  strings.NewReader("yes\n"),
		stdout: &stdout,
		stderr: &stderr,
		check:  fakeUpdateCheck("0.34.0"),
		install: func(_ context.Context, latestVersion string, agent string, _ io.Writer, _ io.Writer) error {
			gotVersion = latestVersion
			gotAgent = agent
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run update: %v", err)
	}
	if !status.Installed || gotVersion != "0.34.0" || gotAgent != "all" {
		t.Fatalf("update not installed correctly: status=%+v version=%q agent=%q", status, gotVersion, gotAgent)
	}
	if !strings.Contains(stderr.String(), "Update now? [y/N]") {
		t.Fatalf("missing confirmation prompt: %q", stderr.String())
	}
}

func TestRunUpdateCommandDeclineSkipsInstall(t *testing.T) {
	restore := setTestVersion("v0.33.1")
	defer restore()
	installed := false
	var stdout, stderr bytes.Buffer
	status, err := runUpdateCommand(context.Background(), nil, updateRuntime{
		stdin:  strings.NewReader("n\n"),
		stdout: &stdout,
		stderr: &stderr,
		check:  fakeUpdateCheck("0.34.0"),
		install: func(context.Context, string, string, io.Writer, io.Writer) error {
			installed = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run update: %v", err)
	}
	if installed || status.Installed || status.Message != "update declined" {
		t.Fatalf("declined update should not install: status=%+v installed=%v", status, installed)
	}
}

func TestRunUpdateCommandYesInstallsSelectedAgent(t *testing.T) {
	restore := setTestVersion("v0.33.1")
	defer restore()
	var gotAgent string
	status, err := runUpdateCommand(context.Background(), []string{"--yes", "--agent=codex"}, updateRuntime{
		stdin:  strings.NewReader(""),
		stdout: io.Discard,
		stderr: io.Discard,
		check:  fakeUpdateCheck("0.34.0"),
		install: func(_ context.Context, _ string, agent string, _ io.Writer, _ io.Writer) error {
			gotAgent = agent
			return nil
		},
	})
	if err != nil {
		t.Fatalf("run update: %v", err)
	}
	if !status.Installed || gotAgent != "codex" {
		t.Fatalf("--yes did not install selected agent: status=%+v agent=%q", status, gotAgent)
	}
}

func TestRunUpdateCommandSurfacesInstallError(t *testing.T) {
	restore := setTestVersion("v0.33.1")
	defer restore()
	want := errors.New("boom")
	_, err := runUpdateCommand(context.Background(), []string{"--yes"}, updateRuntime{
		stdin:  strings.NewReader(""),
		stdout: io.Discard,
		stderr: io.Discard,
		check:  fakeUpdateCheck("0.34.0"),
		install: func(context.Context, string, string, io.Writer, io.Writer) error {
			return want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected install error %v, got %v", want, err)
	}
}

func TestMaybeOfferUpdatePromptsAndInstalls(t *testing.T) {
	restore := setTestVersion("v0.33.1")
	defer restore()
	var stderr bytes.Buffer
	installed := false
	maybeOfferUpdate(context.Background(), []string{"doctor"}, updateRuntime{
		stdin:  strings.NewReader("y\n"),
		stdout: io.Discard,
		stderr: &stderr,
		check:  fakeUpdateCheck("0.34.0"),
		install: func(context.Context, string, string, io.Writer, io.Writer) error {
			installed = true
			return nil
		},
	})
	if !installed {
		t.Fatal("accepted update notice did not run installer")
	}
	if !strings.Contains(stderr.String(), "run 'mnemo update'") || !strings.Contains(stderr.String(), "Update now?") {
		t.Fatalf("notice/prompt missing: %q", stderr.String())
	}
}

func TestUpdateRejectsJSONWithoutCheck(t *testing.T) {
	_, err := runUpdateCommand(context.Background(), []string{"--json"}, updateRuntime{
		stdin:   strings.NewReader(""),
		stdout:  io.Discard,
		stderr:  io.Discard,
		check:   fakeUpdateCheck("0.34.0"),
		install: func(context.Context, string, string, io.Writer, io.Writer) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "--json requires --check") {
		t.Fatalf("expected JSON/check validation error, got %v", err)
	}
}

func fakeUpdateCheck(latest string) func(context.Context, updatecheck.Options) (updatecheck.Result, error) {
	return func(_ context.Context, opts updatecheck.Options) (updatecheck.Result, error) {
		current := updatecheck.Normalize(opts.CurrentVersion)
		return updatecheck.Result{
			Checked:         true,
			UpdateAvailable: updatecheck.IsNewer(latest, current),
			CurrentVersion:  current,
			LatestVersion:   updatecheck.Normalize(latest),
			URL:             "https://example.test/release",
		}, nil
	}
}

func setTestVersion(value string) func() {
	old := version
	version = value
	return func() { version = old }
}
