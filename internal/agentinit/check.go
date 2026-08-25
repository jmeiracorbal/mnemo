package agentinit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
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
	if spec, ok := Spec(agent); ok {
		return spec.Label
	}
	if agent == "" {
		return ""
	}
	return strings.ToUpper(agent[:1]) + agent[1:]
}

// Detected reports whether agent's user-scope directory exists under home.
func Detected(home, agent string) bool {
	spec, ok := Spec(agent)
	if !ok || spec.Detect == nil {
		return false
	}
	return spec.Detect(home)
}

// CheckInstructions reports whether agent's global instructions look installed.
func CheckInstructions(home, agent string) Check {
	spec, ok := Spec(agent)
	if !ok {
		return Check{ID: "global_instructions." + agent, Status: "error", Severity: "error", Message: fmt.Sprintf("unknown agent %q", agent)}
	}
	instruction, ok := globalInstructionSpec(spec)
	if !ok || instruction.Check == nil {
		return Check{ID: "global_instructions." + agent, Status: "error", Severity: "error", Message: fmt.Sprintf("agent %q has no global instruction check", agent)}
	}
	return instruction.Check(home)
}

// CheckMCP reports whether agent's MCP config contains the mnemo server.
func CheckMCP(home, agent string) Check {
	spec, ok := Spec(agent)
	if !ok || spec.MCP.Check == nil {
		return checkError(agent, "mcp_config."+agent, "unknown agent", "")
	}
	return spec.MCP.Check(home)
}

// CheckRuntime reports whether agent's runtime files (hooks/plugin) are present.
// A zero Check means the agent has no runtime surface to report.
func CheckRuntime(home, agent string) Check {
	spec, ok := Spec(agent)
	if !ok {
		return Check{}
	}
	for _, hook := range spec.Hooks {
		if hook.Check == nil {
			continue
		}
		check := hook.Check(home)
		if check.ID != "" {
			return check
		}
	}
	return Check{}
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

func checkJSONMCPWithEnv(path, id, agent, envKey string, keys ...string) Check {
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
	server, ok := jsonPathValue(raw, keys...)
	if !ok {
		return checkWarning(agent, id, "MCP config does not contain mnemo server", path)
	}
	if !jsonPathEquals(server, agent, envKey, "MNEMO_AGENT") {
		return checkWarning(agent, id, "MCP config is missing mnemo provenance environment", path)
	}
	if !jsonMCPInvokesAgentTools(server) {
		return checkWarning(agent, id, "MCP config is missing mnemo agent tool invocation", path)
	}
	return checkOK(agent, id, "MCP config contains mnemo server", path)
}

func jsonMCPInvokesAgentTools(server any) bool {
	m, ok := server.(map[string]any)
	if !ok {
		return false
	}
	command, ok := m["command"]
	if !ok {
		return false
	}
	switch value := command.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return false
		}
		return jsonStringSliceContains(m["args"], "mcp") && jsonStringSliceContains(m["args"], "--tools=agent")
	case []any:
		return jsonStringSliceContains(value, "mcp") && jsonStringSliceContains(value, "--tools=agent")
	default:
		return false
	}
}

func jsonStringSliceContains(value any, want string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if got, ok := item.(string); ok && got == want {
			return true
		}
	}
	return false
}

func checkJSONHookCommands(path, id, agent, okMessage string, eventCommands map[string][]string) Check {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return checkWarning(agent, id, "hooks config not found", path)
	}
	if err != nil {
		return checkError(agent, id, "read hooks config: "+err.Error(), path)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return checkError(agent, id, "parse hooks config JSON: "+err.Error(), path)
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return checkWarning(agent, id, "hooks config does not contain hooks", path)
	}

	var missing []string
	for event, commands := range eventCommands {
		items, ok := hooks[event].([]any)
		if !ok {
			missing = append(missing, event)
			continue
		}
		for _, command := range commands {
			found := false
			for _, item := range items {
				if containsCommand(item, command) {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, event)
				break
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Check{
			ID:       id,
			Status:   "warning",
			Severity: "warning",
			Message:  "hooks config is missing mnemo hook commands",
			Agent:    agent,
			Path:     path,
			Details:  map[string]string{"missing_hooks": strings.Join(missing, ",")},
		}
	}
	return checkOK(agent, id, okMessage, path)
}

func jsonPathValue(v any, keys ...string) (any, bool) {
	current := v
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func jsonPathEquals(v any, want string, keys ...string) bool {
	value, ok := jsonPathValue(v, keys...)
	if !ok {
		return false
	}
	got, ok := value.(string)
	return ok && got == want
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
