package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupStatusOptions struct {
	JSON  bool
	Agent string
	Home  string
}

type setupStatusSummary struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type setupStatusReport struct {
	Status  string             `json:"status"`
	Agent   string             `json:"agent"`
	Summary setupStatusSummary `json:"summary"`
	Rows    []setupStatusRow   `json:"agents"`
}

type setupStatusRow struct {
	Agent        string `json:"agent"`
	Detected     string `json:"detected"`
	MCP          string `json:"mcp"`
	Hooks        string `json:"hooks"`
	Instructions string `json:"instructions"`
}

func runSetup() {
	if len(os.Args) < 3 {
		printSetupUsage()
		os.Exit(1)
	}
	switch os.Args[2] {
	case "status":
		runSetupStatus()
	case "print-config":
		runSetupPrintConfig()
	case "refresh":
		runSetupRefresh()
	case "uninstall":
		runSetupUninstall()
	default:
		if isSetupAgentAlias(os.Args[2]) {
			os.Args = append([]string{os.Args[0], os.Args[1], "refresh", "--agent=" + os.Args[2]}, os.Args[3:]...)
			runSetupRefresh()
			return
		}
		printSetupUsage()
		os.Exit(1)
	}
}

func isSetupAgentAlias(value string) bool {
	for _, agent := range agentinit.SupportedAgents {
		if value == agent {
			return true
		}
	}
	return value == "all"
}

func printSetupUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo setup status [--json] [--agent=AGENT] [--home=DIR]")
	fmt.Fprintln(os.Stderr, "       mnemo setup print-config AGENT [--home=DIR] [--mnemo-bin=PATH]")
	fmt.Fprintln(os.Stderr, "       mnemo setup refresh [--agent=AGENT] [--home=DIR] [--mnemo-bin=PATH]")
	fmt.Fprintln(os.Stderr, "       mnemo setup AGENT [--home=DIR] [--mnemo-bin=PATH]")
	fmt.Fprintln(os.Stderr, "       mnemo setup uninstall --agent=AGENT [--home=DIR]")
}

func runSetupStatus() {
	opts, err := parseSetupStatusArgs(os.Args[3:], os.UserHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup status: home: %v\n", err)
		os.Exit(1)
	}
	report := buildSetupStatusReport(opts)

	if opts.JSON {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo setup status: json: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	} else {
		printSetupStatusReport(report)
	}

	if report.Status == "error" {
		os.Exit(1)
	}
}

func parseSetupStatusArgs(args []string, userHomeDir func() (string, error)) (setupStatusOptions, error) {
	opts := setupStatusOptions{Agent: "all"}
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--agent="):
			opts.Agent = strings.TrimSpace(arg[len("--agent="):])
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		}
	}
	if opts.Agent == "" {
		opts.Agent = "all"
	}
	if opts.Home != "" {
		return opts, nil
	}
	home, err := userHomeDir()
	if err != nil {
		return opts, err
	}
	opts.Home = home
	return opts, nil
}

func buildSetupStatusReport(opts setupStatusOptions) setupStatusReport {
	report := setupStatusReport{Status: "ok", Agent: opts.Agent}
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		report.Status = "error"
		report.Summary.Total = 1
		report.Summary.Errors = 1
		report.Rows = append(report.Rows, setupStatusRow{
			Agent:        opts.Agent,
			Detected:     "error",
			MCP:          "error",
			Hooks:        "error",
			Instructions: err.Error(),
		})
		return report
	}

	for _, agent := range agents {
		report.Rows = append(report.Rows, buildSetupStatusRow(opts.Home, agent))
	}
	report.Summary.Total = len(report.Rows)
	for _, row := range report.Rows {
		if setupStatusRowHasError(row) {
			report.Summary.Errors++
			continue
		}
		if row.Detected == "yes" && row.MCP == "yes" && (row.Hooks == "yes" || row.Hooks == "n/a") && row.Instructions == "yes" {
			report.Summary.OK++
			continue
		}
		report.Summary.Warnings++
	}
	if report.Summary.Errors > 0 {
		report.Status = "error"
	} else if report.Summary.Warnings > 0 {
		report.Status = "warning"
	}
	return report
}

func buildSetupStatusRow(home, agent string) setupStatusRow {
	hooks := statusFromCheck(agentinit.CheckRuntime(home, agent))
	if hooks == "" {
		hooks = "n/a"
	}
	return setupStatusRow{
		Agent:        agentinit.Label(agent),
		Detected:     yesNo(agentinit.Detected(home, agent)),
		MCP:          statusFromCheck(agentinit.CheckMCP(home, agent)),
		Hooks:        hooks,
		Instructions: statusFromCheck(agentinit.CheckInstructions(home, agent)),
	}
}

func printSetupStatusReport(report setupStatusReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Agent\tDetected\tMCP\tHooks\tInstructions")
	for _, row := range report.Rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", row.Agent, row.Detected, row.MCP, row.Hooks, row.Instructions)
	}
	_ = w.Flush()
}

func statusFromCheck(check agentinit.Check) string {
	if check.ID == "" {
		return ""
	}
	switch check.Status {
	case "ok":
		return "yes"
	case "error":
		return "error"
	default:
		return "no"
	}
}

func setupStatusRowHasError(row setupStatusRow) bool {
	return row.Detected == "error" || row.MCP == "error" || row.Hooks == "error" || row.Instructions == "error"
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
