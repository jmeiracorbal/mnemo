package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

type dbMigrateOptions struct {
	DataDir string
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
	default:
		fmt.Fprintf(os.Stderr, "mnemo db: unknown command %q\n", os.Args[2])
		printDBUsage()
		os.Exit(1)
	}
}

func printDBUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo db migrate [--data-dir=DIR] [--check] [--json]")
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
		case strings.HasPrefix(arg, "--data-dir="):
			opts.DataDir = arg[len("--data-dir="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
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
	return dbmigrate.ApplyDataDir(opts.DataDir)
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
