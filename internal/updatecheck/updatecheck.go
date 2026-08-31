package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultEndpoint = "https://api.github.com/repos/jmeiracorbal/mnemo/releases/latest"
	DefaultInterval = 24 * time.Hour
)

type Options struct {
	CurrentVersion string
	HomeDir        string
	Endpoint       string
	Now            func() time.Time
	Client         *http.Client
	Force          bool
}

type Result struct {
	Checked         bool   `json:"checked"`
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	URL             string `json:"url,omitempty"`
	Message         string `json:"message,omitempty"`
}

type cacheFile struct {
	CheckedAt string `json:"checked_at"`
	Latest    string `json:"latest"`
	URL       string `json:"url"`
}

func Check(ctx context.Context, opts Options) (Result, error) {
	current := Normalize(opts.CurrentVersion)
	result := Result{CurrentVersion: current}
	if current == "" || current == "dev" {
		result.Message = "development version; update check skipped"
		return result, nil
	}
	if opts.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return result, fmt.Errorf("home directory: %w", err)
		}
		opts.HomeDir = home
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 800 * time.Millisecond}
	}

	cachePath := filepath.Join(opts.HomeDir, ".mnemo", "update-check.json")
	if !opts.Force {
		if cached, ok := readFreshCache(cachePath, now()); ok {
			result.Checked = true
			result.LatestVersion = Normalize(cached.Latest)
			result.URL = cached.URL
			result.UpdateAvailable = IsNewer(result.LatestVersion, current)
			return result, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mnemo-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("latest release lookup returned %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&payload); err != nil {
		return result, err
	}
	latest := Normalize(payload.TagName)
	result.Checked = true
	result.LatestVersion = latest
	result.URL = payload.HTMLURL
	result.UpdateAvailable = IsNewer(latest, current)
	_ = writeCache(cachePath, cacheFile{CheckedAt: now().UTC().Format(time.RFC3339), Latest: latest, URL: payload.HTMLURL})
	return result, nil
}

func Normalize(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "mnemo ")
	return strings.TrimPrefix(version, "v")
}

func IsNewer(candidate, current string) bool {
	cand := parseVersion(candidate)
	cur := parseVersion(current)
	if len(cand) == 0 || len(cur) == 0 {
		return false
	}
	for i := 0; i < 3; i++ {
		if cand[i] != cur[i] {
			return cand[i] > cur[i]
		}
	}
	return false
}

func readFreshCache(path string, now time.Time) (cacheFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheFile{}, false
	}
	var cached cacheFile
	if err := json.Unmarshal(data, &cached); err != nil {
		return cacheFile{}, false
	}
	checkedAt, err := time.Parse(time.RFC3339, cached.CheckedAt)
	if err != nil || cached.Latest == "" {
		return cacheFile{}, false
	}
	return cached, now.Sub(checkedAt) < DefaultInterval
}

func writeCache(path string, cached cacheFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func parseVersion(version string) [3]int {
	var parsed [3]int
	version = Normalize(version)
	main := strings.SplitN(version, "-", 2)[0]
	parts := strings.Split(main, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return [3]int{}
	}
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			return [3]int{}
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return [3]int{}
		}
		parsed[i] = n
	}
	return parsed
}
