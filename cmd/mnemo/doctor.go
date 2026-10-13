package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	"github.com/jmeiracorbal/mnemo/internal/doctor"
)

func runDoctor() {
	opts, err := parseDoctorArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo doctor: %v\n", err)
		os.Exit(1)
	}
	report := doctor.BuildReport(toDoctorOptions(opts))
	if opts.Fix {
		if err := fixDoctorReport(report, opts, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "mnemo doctor --fix: %v\n", err)
			os.Exit(1)
		}
		report = doctor.BuildReport(toDoctorOptions(opts))
	}

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

func parseDoctorArgs(args []string) (doctorOptions, error) {
	opts := doctorOptions{Agent: "all", Path: "."}
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--fix":
			opts.Fix = true
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--path="):
			opts.Path = arg[len("--path="):]
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		case strings.HasPrefix(arg, "--data-dir="):
			opts.DataDir = arg[len("--data-dir="):]
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	if opts.Agent == "" {
		opts.Agent = "all"
	}
	if opts.Path == "" {
		opts.Path = "."
	}
	return opts, nil
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
		if check.Fix != "" {
			fmt.Printf("           fix: %s\n", check.Fix)
		}
	}
}

// fixDoctorReport repairs every agent with a failing check: it removes legacy
// integrations through the agent's own CLI, then rewrites the agent's mnemo
// configuration with setup refresh. Both steps are idempotent.
func fixDoctorReport(report doctor.Report, opts doctorOptions, out io.Writer) error {
	home := opts.Home
	if home == "" {
		resolved, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home: %w", err)
		}
		home = resolved
	}
	var agents []string
	legacy := map[string]bool{}
	for _, check := range report.Checks {
		if check.Agent == "" || check.Status == "ok" {
			continue
		}
		if !slices.Contains(agents, check.Agent) {
			agents = append(agents, check.Agent)
		}
		if strings.HasPrefix(check.ID, "legacy_plugin.") {
			legacy[check.Agent] = true
		}
	}
	if len(agents) == 0 {
		_, _ = fmt.Fprintln(out, "mnemo doctor --fix: nothing to repair")
		return nil
	}
	for _, agent := range agents {
		if legacy[agent] {
			removed, err := agentinit.RemoveLegacy(home, agent)
			for _, step := range removed {
				_, _ = fmt.Fprintf(out, "mnemo doctor --fix: ran %s\n", step)
			}
			if err != nil {
				return fmt.Errorf("%s: remove legacy plugin: %w", agent, err)
			}
		}
		refreshOpts, err := parseSetupRefreshArgs([]string{"--agent=" + agent, "--home=" + home}, os.UserHomeDir, exec.LookPath)
		if err != nil {
			return err
		}
		result, err := refreshSetup(refreshOpts)
		if err != nil {
			return fmt.Errorf("%s: setup refresh: %w", agent, err)
		}
		for _, path := range result.Updated {
			_, _ = fmt.Fprintf(out, "mnemo doctor --fix: updated %s\n", path)
		}
		if result.ControllerRestarted {
			_, _ = fmt.Fprintln(out, "mnemo doctor --fix: restarted controller service")
		}
	}
	_, _ = fmt.Fprintln(out, "mnemo doctor --fix: restart agent sessions to load the repaired configuration")
	return nil
}
