package doctor

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
)

type Options struct {
	Agent   string
	Path    string
	Home    string
	DataDir string
}

type Report struct {
	Status      string  `json:"status"`
	ProjectRoot string  `json:"project_root"`
	Agent       string  `json:"agent"`
	Summary     Summary `json:"summary"`
	Checks      []Check `json:"checks"`
}

type Summary struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type Check struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Severity string            `json:"severity"`
	Message  string            `json:"message"`
	Agent    string            `json:"agent,omitempty"`
	Path     string            `json:"path,omitempty"`
	Details  map[string]string `json:"details,omitempty"`
}

// BuildReport assembles the mnemo integration diagnostic report.
func BuildReport(opts Options) Report {
	report := Report{Status: "ok", Agent: opts.Agent}
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

	if home != "" && report.ProjectRoot != "" {
		report.addCheck(toCheck(agentinit.CheckCompetingMemory(report.ProjectRoot, home)))
	}

	if home != "" {
		agents, err := agentinit.ExpandAgents(opts.Agent)
		if err != nil {
			report.addCheck(errorCheck("agent", err.Error(), opts.Agent))
		} else {
			for _, agent := range agents {
				report.addCheck(toCheck(agentinit.CheckInstructions(home, agent)))
				report.addCheck(toCheck(agentinit.CheckMCP(home, agent)))
				if check := agentinit.CheckRuntime(home, agent); check.ID != "" {
					report.addCheck(toCheck(check))
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

func (r *Report) addCheck(check Check) {
	r.Checks = append(r.Checks, check)
}

func (r *Report) finalize() {
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

func toCheck(c agentinit.Check) Check {
	return Check{
		ID:       c.ID,
		Status:   c.Status,
		Severity: c.Severity,
		Message:  c.Message,
		Agent:    c.Agent,
		Path:     c.Path,
		Details:  c.Details,
	}
}

func okCheck(id, message, path string) Check {
	return Check{ID: id, Status: "ok", Severity: "info", Message: message, Path: path}
}

func warningCheck(id, message, path string) Check {
	return Check{ID: id, Status: "warning", Severity: "warning", Message: message, Path: path}
}

func errorCheck(id, message, path string) Check {
	return Check{ID: id, Status: "error", Severity: "error", Message: message, Path: path}
}

func checkProjectMarker(root string) Check {
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
	return Check{
		ID: "project_marker", Status: "ok", Severity: "info", Message: ".mnemo marker is valid", Path: path,
		Details: map[string]string{"id": marker.ID, "agents": strings.Join(marker.Agents, ",")},
	}
}

func checkBinaryOnPath() Check {
	path, err := exec.LookPath("mnemo")
	if err != nil {
		return warningCheck("binary_path", "mnemo binary is not available on PATH; plugin integrations may fail", "")
	}
	return okCheck("binary_path", "mnemo binary found on PATH", path)
}

func checkStoreReadOnly(dataDir string) Check {
	status, err := dbmigrate.CheckDataDir(dataDir)
	if err != nil {
		return errorCheck("store", err.Error(), dbmigrate.DBPath(dataDir))
	}
	details := map[string]string{
		"state":           string(status.State),
		"current_version": status.CurrentVersion,
		"latest_version":  status.LatestVersion,
	}
	switch status.State {
	case dbmigrate.StateMissing:
		return Check{ID: "store", Status: "warning", Severity: "warning", Message: status.Message, Path: status.DBPath, Details: details}
	case dbmigrate.StateUpToDate:
		return Check{ID: "store", Status: "ok", Severity: "info", Message: "memory database schema is up to date", Path: status.DBPath, Details: details}
	case dbmigrate.StatePending:
		details["pending"] = strings.TrimSpace(strings.Join(pendingVersions(status.Pending), ","))
		return Check{ID: "store", Status: "warning", Severity: "warning", Message: status.Message, Path: status.DBPath, Details: details}
	default:
		return Check{ID: "store", Status: "error", Severity: "error", Message: status.Message, Path: status.DBPath, Details: details}
	}
}

func pendingVersions(pending []dbmigrate.Migration) []string {
	versions := make([]string, 0, len(pending))
	for _, migration := range pending {
		versions = append(versions, migration.Version)
	}
	return versions
}
