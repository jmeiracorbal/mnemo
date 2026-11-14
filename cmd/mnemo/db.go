package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
	"github.com/jmeiracorbal/mnemo/internal/events"
	"github.com/jmeiracorbal/mnemo/internal/db/recovery"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

type dbMigrateOptions struct {
	DataDir string
	Target  string
	Check   bool
	JSON    bool
}

func runDB() {
	if len(os.Args) < 3 {
		printDBUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "migrate":
		runDBMigrate()
	case "recovery":
		runDBRecovery()
	default:
		fmt.Fprintf(os.Stderr, "mnemo db: unknown command %q\n", os.Args[2])
		printDBUsage()
		os.Exit(1)
	}
}

func printDBUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo db migrate [--data-dir=DIR] [--check] [--target=VERSION] [--json]")
}

func runDBMigrate() {
	opts, err := parseDBMigrateArgs(os.Args[3:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo db migrate: %v\n", err)
		os.Exit(1)
	}
	status, err := migrateDB(opts)
	if opts.JSON {
		out, jsonErr := json.MarshalIndent(status, "", "  ")
		if jsonErr != nil {
			fmt.Fprintf(os.Stderr, "mnemo db migrate: json: %v\n", jsonErr)
			os.Exit(1)
		}
		fmt.Println(string(out))
	} else {
		printDBMigrationStatus(status, opts.Check)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo db migrate: %v\n", err)
		os.Exit(1)
	}
	if opts.Check && status.State != dbmigrate.StateUpToDate {
		os.Exit(2)
	}
}

func parseDBMigrateArgs(args []string) (dbMigrateOptions, error) {
	var opts dbMigrateOptions
	for _, arg := range args {
		switch {
		case arg == "--check":
			opts.Check = true
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--target="):
			opts.Target = strings.TrimPrefix(arg, "--target=")
		case strings.HasPrefix(arg, "--data-dir="):
			opts.DataDir = arg[len("--data-dir="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.Check && opts.Target != "" {
		return opts, fmt.Errorf("--check cannot be combined with --target")
	}
	if opts.DataDir == "" {
		cfg, err := store.DefaultConfig()
		if err != nil {
			return opts, err
		}
		opts.DataDir = cfg.DataDir
	}
	return opts, nil
}

func migrateDB(opts dbMigrateOptions) (dbmigrate.Status, error) {
	if opts.Check {
		return dbmigrate.CheckDataDir(opts.DataDir)
	}
	if err := guardControllerNotRunning(opts.DataDir); err != nil {
		return dbmigrate.Status{}, err
	}
	if opts.Target != "" {
		return dbmigrate.TargetDataDir(opts.DataDir, opts.Target)
	}
	return dbmigrate.ApplyDataDir(opts.DataDir)
}

// guardControllerNotRunning refuses the migration if the controller that owns
// dataDir is reachable. Running migrations while the controller holds the
// database open risks WAL-state conflicts and violates the offline-maintenance
// contract for mnemo db migrate.
//
// The guard distinguishes three states:
//   - config.toml absent → no controller has ever been started → safe
//   - config.toml present but unreadable or invalid → uncertain → fail conservatively
//   - config valid but controller unreachable → consult native service status; fail closed if uncertain
func guardControllerNotRunning(dataDir string) error {
	evtCfg, err := events.LoadConfig(dataDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // no config.toml; no controller has been started for this data dir
		}
		// config.toml exists but is malformed or unreadable — cannot confirm the
		// controller is not running.
		return fmt.Errorf("cannot verify controller state before migrating; inspect config.toml in %s or run 'mnemo db migrate --check': %w", dataDir, err)
	}
	if err := events.CheckHealth(context.Background(), evtCfg); err == nil {
		return fmt.Errorf("the mnemo controller is currently running and owns the database; stop it before migrating")
	}
	// Failed RPC does not establish that the service has stopped. Verify the
	// native user manager before opening or replacing the database.
	home,err:=os.UserHomeDir()
	if err!=nil{return err}
	return confirmManagedControllerStopped(home)
}

func printDBMigrationStatus(status dbmigrate.Status, check bool) {
	verb := "migrate"
	if check {
		verb = "check"
	}
	fmt.Printf("mnemo db %s: %s\n", verb, status.State)
	if status.DBPath != "" {
		fmt.Printf("database: %s\n", status.DBPath)
	}
	if status.CurrentVersion != "" || status.LatestVersion != "" {
		fmt.Printf("version: %s/%s\n", firstNonEmptyString(status.CurrentVersion, "none"), firstNonEmptyString(status.LatestVersion, "none"))
	}
	if len(status.Pending) > 0 {
		fmt.Println("pending migrations:")
		for _, migration := range status.Pending {
			fmt.Printf("  - %s %s\n", migration.Version, migration.Name)
		}
	}
	if status.Message != "" {
		fmt.Println(status.Message)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}


// runDBRecovery is deliberately independent of store.New, because opening a
// Store automatically applies migrations. Recovery must never migrate SQLite.
func runDBRecovery() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "usage: mnemo db recovery <snapshot|restore|verify> --release=VERSION")
		os.Exit(1)
	}
	action := os.Args[3]
	var release string
	for _, arg := range os.Args[4:] {
		if strings.HasPrefix(arg, "--release=") {
			release = strings.TrimPrefix(arg, "--release=")
		} else {
			fmt.Fprintf(os.Stderr, "mnemo db recovery: unknown argument %q\n", arg)
			os.Exit(1)
		}
	}
	cfg, err := store.DefaultConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := executeDBRecovery(context.Background(), cfg.DataDir, release, action); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo db recovery: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("mnemo db recovery: %s %s successful\n", action, release)
}

func executeDBRecovery(ctx context.Context, dataDir, release, action string) error {
	switch action {
	case "snapshot", "restore":
		if err := guardControllerNotRunning(dataDir); err != nil {
			return fmt.Errorf("controller must be stopped for recovery: %w", err)
		}
		if action == "snapshot" {
			_, err := recovery.Snapshot(ctx, dataDir, release)
			return err
		}
		return recovery.Restore(ctx, dataDir, release)
	case "verify":
		return recovery.Verify(ctx, dataDir, release)
	default:
		return fmt.Errorf("unknown recovery action %q", action)
	}
}
