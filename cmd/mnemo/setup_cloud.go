package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/cloudsync"
	"github.com/jmeiracorbal/mnemo/internal/cloudsync/providers/turso"
)

type setupCloudOptions struct {
	NonInteractive bool
	Validate       bool
	Delete         bool
	Provider       string
	URL            string
	Key            string
	ClientID       string
}

func runSetupCloud() {
	opts, err := parseSetupCloudArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup cloud: %v\n", err)
		os.Exit(1)
	}

	if opts.Delete {
		doCloudDelete()
		return
	}

	if opts.Validate {
		doCloudValidate()
		return
	}

	if opts.NonInteractive {
		doCloudSaveNonInteractive(opts)
		return
	}

	doCloudSetupInteractive(opts)
}

func parseSetupCloudArgs(args []string) (setupCloudOptions, error) {
	opts := setupCloudOptions{Provider: "turso"}
	for _, arg := range args {
		switch {
		case arg == "--non-interactive":
			opts.NonInteractive = true
		case arg == "--validate":
			opts.Validate = true
		case arg == "--delete":
			opts.Delete = true
		case strings.HasPrefix(arg, "--provider="):
			opts.Provider = strings.TrimPrefix(arg, "--provider=")
		case strings.HasPrefix(arg, "--url="):
			opts.URL = strings.TrimPrefix(arg, "--url=")
		case strings.HasPrefix(arg, "--key="):
			opts.Key = strings.TrimPrefix(arg, "--key=")
		case strings.HasPrefix(arg, "--client-id="):
			opts.ClientID = strings.TrimPrefix(arg, "--client-id=")
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return opts, nil
}

func doCloudDelete() {
	if err := cloudsync.DeleteFileConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup cloud: delete: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Cloud config deleted.")
}

func doCloudValidate() {
	cfg, err := cloudsync.LoadFileConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup cloud: read config: %v\n", err)
		os.Exit(1)
	}
	// Also merge env vars.
	envCfg, _ := cloudsync.ConfigFromEnv()
	if cfg.URL == "" {
		cfg = envCfg
	}
	validated, err := cfg.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Config invalid: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Testing connection (%s @ %s)...", validated.Provider, validated.URL)
	if err := (turso.Provider{}).Ping(validated); err != nil {
		fmt.Printf(" ✗\n")
		fmt.Fprintf(os.Stderr, "✗ Connection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(" ✓\n")
	fmt.Println("✓ Cloud credentials are valid.")
}

func doCloudSaveNonInteractive(opts setupCloudOptions) {
	if opts.URL == "" || opts.Key == "" || opts.ClientID == "" {
		fmt.Fprintln(os.Stderr, "mnemo setup cloud --non-interactive requires --url=, --key=, and --client-id=")
		os.Exit(1)
	}
	cfg := cloudsync.Config{
		Provider: opts.Provider,
		URL:      opts.URL,
		Key:      opts.Key,
		ClientID: opts.ClientID,
	}
	validated, err := cfg.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Config invalid: %v\n", err)
		os.Exit(1)
	}
	if err := cloudsync.SaveFileConfig(validated); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Save failed: %v\n", err)
		os.Exit(1)
	}
	if err := (turso.Provider{}).Ping(validated); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Connection failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Config saved but connection test failed. Run 'mnemo setup cloud --validate' to retest.")
		os.Exit(1)
	}
	fmt.Println("✓ Cloud config saved and connection verified.")
}

func doCloudSetupInteractive(opts setupCloudOptions) {
	r := bufio.NewReader(os.Stdin)

	fmt.Println("Cloud sync setup for mnemo.")
	fmt.Println()

	// Load existing values as defaults.
	existing, _ := cloudsync.LoadFileConfig()

	provider := promptWithDefault(r, "Provider [turso]", firstNonEmpty(opts.Provider, existing.Provider, "turso"))
	if provider != "turso" {
		fmt.Fprintf(os.Stderr, "✗ unknown cloud provider %q\n", provider)
		os.Exit(1)
	}

	dbURL := promptWithDefault(r, "Database URL (libsql://...)", firstNonEmpty(opts.URL, existing.URL))
	authKey := promptWithDefault(r, "Auth token", firstNonEmpty(opts.Key, existing.Key))
	clientID := promptWithDefault(r, "Client ID", firstNonEmpty(opts.ClientID, existing.ClientID, defaultClientID()))

	cfg := cloudsync.Config{
		Provider: provider,
		URL:      dbURL,
		Key:      authKey,
		ClientID: clientID,
	}
	validated, err := cfg.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Config invalid: %v\n", err)
		os.Exit(1)
	}

	path, _ := cloudsync.CloudConfigPath()
	fmt.Printf("\nConfig will be saved to %s\n", path)
	answer := promptWithDefault(r, "Save?", "Y")
	if !strings.EqualFold(strings.TrimSpace(answer), "y") {
		fmt.Println("Aborted.")
		return
	}

	if err := cloudsync.SaveFileConfig(validated); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Save failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Config saved.")

	fmt.Printf("  Testing connection (%s @ %s)...", validated.Provider, validated.URL)
	if err := (turso.Provider{}).Ping(validated); err != nil {
		fmt.Printf(" ✗\n")
		fmt.Fprintf(os.Stderr, "Connection test failed: %v\n", err)
		fmt.Println("Config saved. Run 'mnemo setup cloud --validate' to retest, or 'mnemo setup cloud --delete' to remove.")
		return
	}
	fmt.Printf(" ✓\n")
	fmt.Printf("✓ Connected (%s @ %s)\n\n", validated.Provider, validated.URL)
	fmt.Println("Run 'mnemo sync' to start syncing.")
}

func promptWithDefault(r *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("? %s [%s]: ", label, def)
	} else {
		fmt.Printf("? %s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultClientID() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}
