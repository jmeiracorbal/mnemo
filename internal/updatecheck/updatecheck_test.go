package updatecheck

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"v0.34.0", "v0.33.2", true},
		{"0.33.3", "0.33.2", true},
		{"0.33.2", "0.33.2", false},
		{"0.32.9", "0.33.2", false},
		{"dev", "0.33.2", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.candidate, tc.current); got != tc.want {
			t.Fatalf("IsNewer(%q, %q) = %v, want %v", tc.candidate, tc.current, got, tc.want)
		}
	}
}

func TestCheckFetchesAndCachesLatestRelease(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.34.0","html_url":"https://example.test/release"}`)),
		}, nil
	})}
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	home := t.TempDir()
	result, err := Check(context.Background(), Options{CurrentVersion: "v0.33.2", HomeDir: home, Endpoint: "https://example.test/latest", Client: client, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !result.UpdateAvailable || result.LatestVersion != "0.34.0" || calls != 1 {
		t.Fatalf("unexpected result=%+v calls=%d", result, calls)
	}
	result, err = Check(context.Background(), Options{CurrentVersion: "v0.33.2", HomeDir: home, Endpoint: "https://example.test/latest", Client: client, Now: func() time.Time { return now.Add(time.Hour) }})
	if err != nil {
		t.Fatalf("cached check update: %v", err)
	}
	if !result.UpdateAvailable || calls != 1 {
		t.Fatalf("expected cached result without another call, result=%+v calls=%d", result, calls)
	}
	if _, err := os.Stat(filepath.Join(home, ".mnemo", "update-check.json")); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
}

func TestCheckSkipsDevVersion(t *testing.T) {
	result, err := Check(context.Background(), Options{CurrentVersion: "dev", HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("dev check: %v", err)
	}
	if result.Checked || result.UpdateAvailable {
		t.Fatalf("dev version should not check: %+v", result)
	}
}

func TestShouldCheckSkipsMachineAndAgentPaths(t *testing.T) {
	fakeTTY := os.Stderr
	getenv := func(key string) string { return "" }
	if ShouldCheck("dev", []string{"save"}, fakeTTY, getenv) {
		t.Fatal("dev should skip")
	}
	if ShouldCheck("0.33.2", []string{"mcp"}, fakeTTY, getenv) {
		t.Fatal("mcp should skip")
	}
	if ShouldCheck("0.33.2", []string{"doctor", "--json"}, fakeTTY, getenv) {
		t.Fatal("json output should skip")
	}
	agentEnv := func(key string) string {
		if key == "MNEMO_SOURCE" {
			return "hook"
		}
		return ""
	}
	if ShouldCheck("0.33.2", []string{"save"}, fakeTTY, agentEnv) {
		t.Fatal("hook source should skip")
	}
}
