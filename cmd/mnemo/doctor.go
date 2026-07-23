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
				report.addCheck(checkGlobalInstructions(home, agent))
				report.addCheck(checkMCPConfig(home, agent))
				if check := checkAgentRuntimeFiles(home, agent); check.ID != "" {
					report.addCheck(check)
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

func checkGlobalInstructions(home, agent string) doctorCheck {
	path, err := agentinit.GlobalInstructionPath(home, agent)
	if err != nil {
		return errorCheck("global_instructions."+agent, err.Error(), "")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return agentWarning(agent, "global_instructions."+agent, "global instructions not found", path)
	}
	if err != nil {
		return agentError(agent, "global_instructions."+agent, "read global instructions: "+err.Error(), path)
	}
	content := string(data)
	if !strings.Contains(content, ".mnemo") || !strings.Contains(content, "ONLY persistent memory system") {
		return agentWarning(agent, "global_instructions."+agent, "global instructions do not look like mnemo instructions", path)
	}
	if agent != "cursor" && (!strings.Contains(content, "<!-- mnemo:start -->") || !strings.Contains(content, "<!-- mnemo:end -->")) {
		return agentWarning(agent, "global_instructions."+agent, "global instructions are missing mnemo managed markers", path)
	}
	return agentOK(agent, "global_instructions."+agent, "global instructions installed", path)
}

func checkMCPConfig(home, agent string) doctorCheck {
	switch agent {
	case "claudecode":
		path := filepath.Join(home, ".claude", ".mcp.json")
		return checkJSONHas(path, "mcp_config."+agent, agent, "mcpServers", "mnemo")
	case "cursor":
		path := filepath.Join(home, ".cursor", "mcp.json")
		return checkJSONHas(path, "mcp_config."+agent, agent, "mcpServers", "mnemo")
	case "windsurf":
		path := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
		return checkJSONHas(path, "mcp_config."+agent, agent, "mcpServers", "mnemo")
	case "codex":
		path := filepath.Join(home, ".codex", "config.toml")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return agentWarning(agent, "mcp_config."+agent, "MCP config not found", path)
		}
		if err != nil {
			return agentError(agent, "mcp_config."+agent, "read MCP config: "+err.Error(), path)
		}
		content := string(data)
		if !strings.Contains(content, "[mcp_servers.mnemo]") || !strings.Contains(content, "mcp") {
			return agentWarning(agent, "mcp_config."+agent, "MCP config does not contain mnemo server", path)
		}
		return agentOK(agent, "mcp_config."+agent, "MCP config contains mnemo server", path)
	case "opencode":
		path := filepath.Join(home, ".config", "opencode", "opencode.json")
		return checkJSONHas(path, "mcp_config."+agent, agent, "mcp", "mnemo")
	default:
		return agentError(agent, "mcp_config."+agent, "unknown agent", "")
	}
}

func checkAgentRuntimeFiles(home, agent string) doctorCheck {
	switch agent {
	case "cursor":
		paths := []string{
			filepath.Join(home, ".cursor", "hooks.json"),
			filepath.Join(home, ".cursor", "hooks", "before-submit-prompt.sh"),
			filepath.Join(home, ".cursor", "hooks", "stop.sh"),
		}
		return checkFiles(agent, "runtime_files."+agent, "Cursor global hooks installed", paths, true)
	case "windsurf":
		paths := []string{
			filepath.Join(home, ".codeium", "windsurf", "hooks.json"),
			filepath.Join(home, ".codeium", "windsurf", "hooks", "pre-user-prompt.sh"),
			filepath.Join(home, ".codeium", "windsurf", "hooks", "post-cascade-response.sh"),
		}
		return checkFiles(agent, "runtime_files."+agent, "Windsurf global hooks installed", paths, true)
	case "codex":
		paths := []string{
			filepath.Join(home, ".codex", "hooks.json"),
			filepath.Join(home, ".codex", "hooks", "session-start.sh"),
			filepath.Join(home, ".codex", "hooks", "stop.sh"),
			filepath.Join(home, ".codex", "hooks", "mnemo-protocol.md"),
		}
		return checkFiles(agent, "runtime_files."+agent, "Codex global hooks installed", paths, true)
	case "opencode":
		paths := []string{
			filepath.Join(home, ".config", "opencode", "plugins", "mnemo.ts"),
			filepath.Join(home, ".config", "opencode", "plugins", "mnemo-protocol.md"),
		}
		return checkFiles(agent, "runtime_files."+agent, "OpenCode plugin files installed", paths, false)
	default:
		return doctorCheck{}
	}
}

func checkFiles(agent, id, okMessage string, paths []string, executableScripts bool) doctorCheck {
	missing := []string{}
	notExecutable := []string{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, path)
				continue
			}
			return agentError(agent, id, "stat runtime file: "+err.Error(), path)
		}
		if info.IsDir() {
			missing = append(missing, path)
			continue
		}
		if executableScripts && strings.HasSuffix(path, ".sh") && info.Mode()&0111 == 0 {
			notExecutable = append(notExecutable, path)
		}
	}
	if len(missing) > 0 || len(notExecutable) > 0 {
		details := map[string]string{}
		if len(missing) > 0 {
			details["missing"] = strings.Join(missing, ",")
		}
		if len(notExecutable) > 0 {
			details["not_executable"] = strings.Join(notExecutable, ",")
		}
		return doctorCheck{ID: id, Status: "warning", Severity: "warning", Agent: agent, Message: "runtime files are incomplete", Details: details}
	}
	return agentOK(agent, id, okMessage, strings.Join(paths, ","))
}

func checkJSONHas(path, id, agent string, keys ...string) doctorCheck {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return agentWarning(agent, id, "MCP config not found", path)
	}
	if err != nil {
		return agentError(agent, id, "read MCP config: "+err.Error(), path)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return agentError(agent, id, "parse MCP config JSON: "+err.Error(), path)
	}
	if !jsonPathExists(raw, keys...) {
		return agentWarning(agent, id, "MCP config does not contain mnemo server", path)
	}
	return agentOK(agent, id, "MCP config contains mnemo server", path)
}

func jsonPathExists(v any, keys ...string) bool {
	current := v
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = m[key]
		if !ok {
			return false
		}
	}
	return true
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

func agentOK(agent, id, message, path string) doctorCheck {
	check := okCheck(id, message, path)
	check.Agent = agent
	return check
}

func agentWarning(agent, id, message, path string) doctorCheck {
	check := warningCheck(id, message, path)
	check.Agent = agent
	return check
}

func agentError(agent, id, message, path string) doctorCheck {
	check := errorCheck(id, message, path)
	check.Agent = agent
	return check
}
