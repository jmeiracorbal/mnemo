package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupStatusOptions struct {
	JSON  bool
	Agent string
	Home  string
}

type setupStatusReport struct {
	Status  string           `json:"status"`
	Agent   string           `json:"agent"`
	Summary doctorSummary    `json:"summary"`
	Rows    []setupStatusRow `json:"agents"`
}

type setupStatusRow struct {
	Agent        string `json:"agent"`
	Detected     string `json:"detected"`
	MCP          string `json:"mcp"`
	Hooks        string `json:"hooks"`
	Instructions string `json:"instructions"`
}

func runSetup() {
	if len(os.Args) < 3 || os.Args[2] != "status" {
		fmt.Fprintln(os.Stderr, "usage: mnemo setup status [--json] [--agent=AGENT] [--home=DIR]")
		os.Exit(1)
	}

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
	hooks := statusFromCheck(checkAgentRuntimeFiles(home, agent))
	if hooks == "" {
		hooks = "n/a"
	}
	return setupStatusRow{
		Agent:        agentLabel(agent),
		Detected:     yesNo(agentDetected(home, agent)),
		MCP:          statusFromCheck(checkMCPConfig(home, agent)),
		Hooks:        hooks,
		Instructions: statusFromCheck(checkGlobalInstructions(home, agent)),
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

func statusFromCheck(check doctorCheck) string {
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

func agentDetected(home, agent string) bool {
	for _, path := range agentDetectionPaths(home, agent) {
		if pathExists(path) {
			return true
		}
	}
	return false
}

func agentDetectionPaths(home, agent string) []string {
	switch agent {
	case "claudecode":
		return []string{filepath.Join(home, ".claude")}
	case "cursor":
		return []string{filepath.Join(home, ".cursor")}
	case "windsurf":
		return []string{filepath.Join(home, ".codeium", "windsurf")}
	case "codex":
		return []string{filepath.Join(home, ".codex")}
	case "opencode":
		return []string{filepath.Join(home, ".config", "opencode")}
	default:
		return nil
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func agentLabel(agent string) string {
	switch agent {
	case "claudecode":
		return "Claude"
	case "opencode":
		return "OpenCode"
	default:
		return strings.ToUpper(agent[:1]) + agent[1:]
	}
}
