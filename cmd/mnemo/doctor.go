package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/doctor"
)

func runDoctor() {
	opts := parseDoctorArgs(os.Args[2:])
	report := doctor.BuildReport(toDoctorOptions(opts))

	if opts.JSON {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo doctor: json: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	} else {
		printDoctorReport(report)
	}

	if report.Status == "error" {
		os.Exit(1)
	}
}

func parseDoctorArgs(args []string) doctorOptions {
	opts := doctorOptions{Agent: "all", Path: "."}
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--path="):
			opts.Path = arg[len("--path="):]
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		case strings.HasPrefix(arg, "--data-dir="):
			opts.DataDir = arg[len("--data-dir="):]
		}
	}
	if opts.Agent == "" {
		opts.Agent = "all"
	}
	if opts.Path == "" {
		opts.Path = "."
	}
	return opts
}

func toDoctorOptions(opts doctorOptions) doctor.Options {
	return doctor.Options{
		Agent:   opts.Agent,
		Path:    opts.Path,
		Home:    opts.Home,
		DataDir: opts.DataDir,
	}
}

func printDoctorReport(report doctor.Report) {
	fmt.Printf("mnemo doctor: %s (%d ok, %d warnings, %d errors)\n", report.Status, report.Summary.OK, report.Summary.Warnings, report.Summary.Errors)
	if report.ProjectRoot != "" {
		fmt.Printf("project: %s\n", report.ProjectRoot)
	}
	fmt.Println()
	for _, check := range report.Checks {
		marker := "✓"
		switch check.Status {
		case "warning":
			marker = "!"
		case "error":
			marker = "✗"
		}
		suffix := ""
		if check.Agent != "" {
			suffix += " [" + check.Agent + "]"
		}
		if check.Path != "" {
			suffix += " " + check.Path
		}
		fmt.Printf("%s %-8s %s%s\n", marker, check.Status, check.Message, suffix)
	}
}
