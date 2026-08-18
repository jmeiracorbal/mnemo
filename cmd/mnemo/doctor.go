package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	_ "modernc.org/sqlite"
)

type doctorOptions struct {
	JSON    bool
	Agent   string
	Path    string
	Home    string
	DataDir string
}

type doctorReport struct {
	Status      string        `json:"status"`
	ProjectRoot string        `json:"project_root"`
	Agent       string        `json:"agent"`
	Summary     doctorSummary `json:"summary"`
	Checks      []doctorCheck `json:"checks"`
}

type doctorSummary struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type doctorCheck struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Severity string            `json:"severity"`
	Message  string            `json:"message"`
	Agent    string            `json:"agent,omitempty"`
	Path     string            `json:"path,omitempty"`
	Details  map[string]string `json:"details,omitempty"`
}

func runDoctor() {
	opts := parseDoctorArgs(os.Args[2:])
	report := buildDoctorReport(opts)

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

func buildDoctorReport(opts doctorOptions) doctorReport {
	report := doctorReport{Status: "ok", Agent: opts.Agent}
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		report.addCheck(errorCheck("project_path", "resolve project path: "+err.Error(), opts.Path))
	} else {
		report.ProjectRoot = agentinit.ProjectRoot(absPath)
		report.addCheck(okCheck("project_path", "project path resolved", report.ProjectRoot))
		report.addCheck(checkProjectMarker(report.ProjectRoot))
	}

	home := opts.Home
	if home == "" {
		resolved, err := os.UserHomeDir()
		if err != nil {
			report.addCheck(errorCheck("home", "resolve home directory: "+err.Error(), ""))
		} else {
			home = resolved
			report.addCheck(okCheck("home", "home directory resolved", home))
		}
	} else {
		report.addCheck(okCheck("home", "home directory override provided", home))
	}

	report.addCheck(checkBinaryOnPath())

	if home != "" {
		agents, err := agentinit.ExpandAgents(opts.Agent)
		if err != nil {
			report.addCheck(errorCheck("agent", err.Error(), opts.Agent))
		} else {
			for _, agent := range agents {
				report.addCheck(toDoctorCheck(agentinit.CheckInstructions(home, agent)))
				report.addCheck(toDoctorCheck(agentinit.CheckMCP(home, agent)))
				if check := agentinit.CheckRuntime(home, agent); check.ID != "" {
					report.addCheck(toDoctorCheck(check))
				}
			}
		}
	}

	if opts.DataDir == "" && home != "" {
		opts.DataDir = filepath.Join(home, ".mnemo")
	}
	if opts.DataDir != "" {
		report.addCheck(checkStoreReadOnly(opts.DataDir))
	}

	report.finalize()
	return report
}

func (r *doctorReport) addCheck(check doctorCheck) {
	r.Checks = append(r.Checks, check)
}

func (r *doctorReport) finalize() {
	for _, check := range r.Checks {
		r.Summary.Total++
		switch check.Status {
		case "ok":
			r.Summary.OK++
		case "warning":
			r.Summary.Warnings++
		case "error":
			r.Summary.Errors++
		}
	}
	if r.Summary.Errors > 0 {
		r.Status = "error"
	} else if r.Summary.Warnings > 0 {
		r.Status = "warning"
	} else {
		r.Status = "ok"
	}
}

func printDoctorReport(report doctorReport) {
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

func toDoctorCheck(c agentinit.Check) doctorCheck {
	return doctorCheck{
		ID:       c.ID,
		Status:   c.Status,
		Severity: c.Severity,
		Message:  c.Message,
		Agent:    c.Agent,
		Path:     c.Path,
		Details:  c.Details,
	}
}

func okCheck(id, message, path string) doctorCheck {
	return doctorCheck{ID: id, Status: "ok", Severity: "info", Message: message, Path: path}
}

func warningCheck(id, message, path string) doctorCheck {
	return doctorCheck{ID: id, Status: "warning", Severity: "warning", Message: message, Path: path}
}

func errorCheck(id, message, path string) doctorCheck {
	return doctorCheck{ID: id, Status: "error", Severity: "error", Message: message, Path: path}
}

func checkProjectMarker(root string) doctorCheck {
	path := filepath.Join(root, ".mnemo")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return warningCheck("project_marker", ".mnemo not found; run mnemo init to activate this project", path)
	}
	if err != nil {
		return errorCheck("project_marker", "read .mnemo: "+err.Error(), path)
	}
	var marker agentinit.Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return errorCheck("project_marker", "malformed .mnemo: "+err.Error(), path)
	}
	if strings.TrimSpace(marker.ID) == "" {
		return errorCheck("project_marker", ".mnemo has no project id", path)
	}
	return doctorCheck{ID: "project_marker", Status: "ok", Severity: "info", Message: ".mnemo marker is valid", Path: path, Details: map[string]string{"id": marker.ID, "agents": strings.Join(marker.Agents, ",")}}
}

func checkBinaryOnPath() doctorCheck {
	path, err := exec.LookPath("mnemo")
	if err != nil {
		return warningCheck("binary_path", "mnemo binary is not available on PATH; plugin integrations may fail", "")
	}
	return okCheck("binary_path", "mnemo binary found on PATH", path)
}

func checkStoreReadOnly(dataDir string) doctorCheck {
	dbPath := filepath.Join(dataDir, "memory.db")
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return warningCheck("store", "memory database not found yet", dbPath)
	} else if err != nil {
		return errorCheck("store", "stat memory database: "+err.Error(), dbPath)
	}
	db, err := sql.Open("sqlite", sqliteReadOnlyDBURI(dbPath))
	if err != nil {
		return errorCheck("store", "open memory database read-only: "+err.Error(), dbPath)
	}
	defer func() {
		_ = db.Close()
	}()
	if err := db.Ping(); err != nil {
		return errorCheck("store", "ping memory database read-only: "+err.Error(), dbPath)
	}
	counts := map[string]string{}
	queries := map[string]string{
		"sessions":     "SELECT COUNT(*) FROM sessions",
		"observations": "SELECT COUNT(*) FROM observations",
		"user_prompts": "SELECT COUNT(*) FROM user_prompts",
	}
	for table, query := range queries {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			return errorCheck("store", "query "+table+": "+err.Error(), dbPath)
		}
		counts[table] = fmt.Sprintf("%d", count)
	}
	return doctorCheck{ID: "store", Status: "ok", Severity: "info", Message: "memory database opens read-only", Path: dbPath, Details: counts}
}

func sqliteReadOnlyDBURI(dbPath string) string {
	return "file:" + filepath.ToSlash(dbPath) + "?mode=ro&immutable=1"
}
