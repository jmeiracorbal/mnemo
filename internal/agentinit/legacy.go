package agentinit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// legacyCommand is an agent CLI invocation that removes a legacy mnemo
// integration. Setup never edits an agent's plugin registry directly.
type legacyCommand struct {
	Name string
	Args []string
	// Dir is the working directory for project-scoped removals.
	Dir string
}

func (c legacyCommand) String() string {
	command := c.Name + " " + strings.Join(c.Args, " ")
	if c.Dir != "" {
		return "cd " + shellQuote(c.Dir) + " && " + command
	}
	return command
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// legacyCheck reports a legacy mnemo plugin whose removal commands are
// commands; none means the agent is clean.
func legacyCheck(agent, label string, commands []legacyCommand, path string) Check {
	id := "legacy_plugin." + agent
	if len(commands) == 0 {
		return checkOK(agent, id, "no legacy mnemo plugin installed", path)
	}
	manual := make([]string, len(commands))
	for i, cmd := range commands {
		manual[i] = cmd.String()
	}
	check := checkWarning(agent, id, "legacy mnemo "+label+" plugin is installed and registers mnemo a second time", path)
	check.Fix = "mnemo doctor --fix --agent=" + agent
	check.Details = map[string]string{"commands": strings.Join(manual, " && ")}
	return check
}

// runLegacyCommands runs each removal through the agent's own CLI and returns
// a description of what it removed.
func runLegacyCommands(home string, commands []legacyCommand) ([]string, error) {
	var removed []string
	for _, c := range commands {
		bin, err := exec.LookPath(c.Name)
		if err != nil {
			return removed, fmt.Errorf("%s is not on PATH; run manually: %s", c.Name, c)
		}
		cmd := exec.Command(bin, c.Args...)
		cmd.Dir = c.Dir
		cmd.Env = append(os.Environ(), "HOME="+home)
		cmd.Stdin = nil
		if out, err := cmd.CombinedOutput(); err != nil {
			return removed, fmt.Errorf("%s: %w: %s", c, err, strings.TrimSpace(string(out)))
		}
		removed = append(removed, c.String())
	}
	return removed, nil
}
