package agentinit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Check is a diagnostic result for one agent-owned concern.
type Check struct {
	ID       string
	Status   string
	Severity string
	Message  string
	Agent    string
	Path     string
	Details  map[string]string
}

// Label returns the display name owned by agent.
func Label(agent string) string {
	switch agent {
	case "claudecode":
		return claudeCodeLabel()
	case "cursor":
		return cursorLabel()
	case "windsurf":
		return windsurfLabel()
	case "codex":
		return codexLabel()
	case "opencode":
		return openCodeLabel()
	default:
		return strings.ToUpper(agent[:1]) + agent[1:]
	}
}

// Detected reports whether agent's user-scope directory exists under home.
func Detected(home, agent string) bool {
	for _, path := range detectionPaths(home, agent) {
		if pathExists(path) {
			return true
		}
	}
	return false
}

func detectionPaths(home, agent string) []string {
	switch agent {
	case "claudecode":
		return claudeCodeDetectionPaths(home)
	case "cursor":
		return cursorDetectionPaths(home)
	case "windsurf":
		return windsurfDetectionPaths(home)
	case "codex":
		return codexDetectionPaths(home)
	case "opencode":
		return openCodeDetectionPaths(home)
	default:
		return nil
	}
}

// CheckInstructions reports whether agent's global instructions look installed.
func CheckInstructions(home, agent string) Check {
	switch agent {
	case "claudecode":
		return claudeCodeCheckInstructions(home)
	case "cursor":
		return cursorCheckInstructions(home)
	case "windsurf":
		return windsurfCheckInstructions(home)
	case "codex":
		return codexCheckInstructions(home)
	case "opencode":
		return openCodeCheckInstructions(home)
	default:
		return Check{ID: "global_instructions." + agent, Status: "error", Severity: "error", Message: fmt.Sprintf("unknown agent %q", agent)}
	}
}

// CheckMCP reports whether agent's MCP config contains the mnemo server.
func CheckMCP(home, agent string) Check {
	switch agent {
	case "claudecode":
		return claudeCodeCheckMCP(home)
	case "cursor":
		return cursorCheckMCP(home)
	case "windsurf":
		return windsurfCheckMCP(home)
	case "codex":
		return codexCheckMCP(home)
	case "opencode":
		return openCodeCheckMCP(home)
	default:
		return checkError(agent, "mcp_config."+agent, "unknown agent", "")
	}
}

// CheckRuntime reports whether agent's runtime files (hooks/plugin) are present.
// A zero Check means the agent has no runtime surface to report.
func CheckRuntime(home, agent string) Check {
	switch agent {
	case "claudecode":
		return claudeCodeCheckRuntime(home)
	case "cursor":
		return cursorCheckRuntime(home)
	case "windsurf":
		return windsurfCheckRuntime(home)
	case "codex":
		return codexCheckRuntime(home)
	case "opencode":
		return openCodeCheckRuntime(home)
	default:
		return Check{}
	}
}

func checkInstructionFile(agent, path string, requireMarkers bool) Check {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return checkWarning(agent, "global_instructions."+agent, "global instructions not found", path)
	}
	if err != nil {
		return checkError(agent, "global_instructions."+agent, "read global instructions: "+err.Error(), path)
	}
	content := string(data)
	if !strings.Contains(content, ".mnemo") || !strings.Contains(content, "ONLY persistent memory system") {
		return checkWarning(agent, "global_instructions."+agent, "global instructions do not look like mnemo instructions", path)
	}
	if requireMarkers && (!strings.Contains(content, "<!-- mnemo:start -->") || !strings.Contains(content, "<!-- mnemo:end -->")) {
		return checkWarning(agent, "global_instructions."+agent, "global instructions are missing mnemo managed markers", path)
	}
	return checkOK(agent, "global_instructions."+agent, "global instructions installed", path)
}

func checkFiles(agent, id, okMessage string, paths []string, executableScripts bool) Check {
	missing := []string{}
	notExecutable := []string{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, path)
				continue
			}
			return checkError(agent, id, "stat runtime file: "+err.Error(), path)
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
		return Check{ID: id, Status: "warning", Severity: "warning", Agent: agent, Message: "runtime files are incomplete", Details: details}
	}
	return checkOK(agent, id, okMessage, strings.Join(paths, ","))
}

func checkJSONHas(path, id, agent string, keys ...string) Check {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return checkWarning(agent, id, "MCP config not found", path)
	}
	if err != nil {
		return checkError(agent, id, "read MCP config: "+err.Error(), path)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return checkError(agent, id, "parse MCP config JSON: "+err.Error(), path)
	}
	if !jsonPathExists(raw, keys...) {
		return checkWarning(agent, id, "MCP config does not contain mnemo server", path)
	}
	return checkOK(agent, id, "MCP config contains mnemo server", path)
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

func checkOK(agent, id, message, path string) Check {
	return Check{ID: id, Status: "ok", Severity: "info", Message: message, Agent: agent, Path: path}
}

func checkWarning(agent, id, message, path string) Check {
	return Check{ID: id, Status: "warning", Severity: "warning", Message: message, Agent: agent, Path: path}
}

func checkError(agent, id, message, path string) Check {
	return Check{ID: id, Status: "error", Severity: "error", Message: message, Agent: agent, Path: path}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
