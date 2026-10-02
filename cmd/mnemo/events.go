package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo-adapters/agents"
	"github.com/jmeiracorbal/mnemo-adapters/mapping"
	"github.com/jmeiracorbal/mnemo-events"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

// runEvents only publishes to the controller. It deliberately does not open a
// Store: hook processes must never touch SQLite, even to initialize it.
func runEvents() {
	args := eventArgs()
	if len(args) == 0 {
		eventUsage()
		return
	}
	command := args[0]
	values, err := eventFlagValues(args[1:])
	if err != nil {
		eventFail(err)
	}

	if command == "map" {
		data, err := mapping.EventMapJSON(agents.Agent(values["agent"]))
		if err != nil {
			eventFail(err)
		}
		fmt.Println(string(data))
		return
	}

	project, directory := values["project"], values["directory"]
	if project == "" || directory == "" {
		eventFail(fmt.Errorf("--project and --directory are required"))
	}
	if err := eventValidateMarker(project, directory); err != nil {
		eventFail(err)
	}
	storeConfig, err := store.DefaultConfig()
	if err != nil {
		eventFail(err)
	}
	cfg, err := events.LoadConfig(storeConfig.DataDir)
	if err != nil {
		eventFail(err)
	}

	switch command {
	case "publish":
		payload := json.RawMessage(values["payload"])
		event := events.Event{ID: uuid.NewString(), Type: values["type"], Project: project, ExecutionKey: values["execution-key"], Agent: agents.Agent(values["agent"]), NativeID: values["native-id"], Payload: payload, OccurredAt: time.Now().UTC()}
		if err := event.Validate(); err != nil {
			eventFail(err)
		}
		if err := events.Publish(context.Background(), cfg, event); err != nil {
			eventFail(err)
		}
	case "invoke":
		payload := json.RawMessage(values["payload"])
		if !json.Valid(payload) {
			eventFail(fmt.Errorf("--payload must be valid JSON"))
		}
		if values["agent"] == "" || values["native-id"] == "" || values["tool"] == "" {
			eventFail(fmt.Errorf("--agent, --native-id, and --tool are required"))
		}
		var result struct {
			Text string `json:"text"`
		}
		input := struct {
			Agent     agents.Agent    `json:"agent"`
			NativeID  string          `json:"native_id"`
			Project   string          `json:"project"`
			Directory string          `json:"directory"`
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}{agents.Agent(values["agent"]), values["native-id"], project, directory, values["tool"], payload}
		if err := events.Call(context.Background(), cfg, "agent_tool", input, &result); err != nil {
			eventFail(err)
		}
		fmt.Println(result.Text)
	default:
		eventUsage()
	}
}

func eventFail(err error) {
	fmt.Fprintln(os.Stderr, "mnemo events:", err)
	os.Exit(1)
}

// eventValidateMarker is the Capa-1 deterministic guardrail for the events
// path (Pi, OpenCode). It confirms that directory (or its git root) contains a
// .mnemo marker whose id matches project before any event or invoke call
// reaches the controller over NATS.
func eventValidateMarker(project, directory string) error {
	root := eventGitRoot(directory)
	data, err := os.ReadFile(filepath.Join(root, ".mnemo"))
	if err != nil {
		return fmt.Errorf("project not initialized at %s — run 'mnemo init' to activate persistent memory", root)
	}
	var m struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &m); err != nil || strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("invalid .mnemo marker at %s — run 'mnemo init'", root)
	}
	if m.ID != project {
		return fmt.Errorf("project id mismatch — .mnemo has %q but --project is %q", m.ID, project)
	}
	return nil
}

func eventGitRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	return strings.TrimSpace(string(out))
}

func eventArgs() []string {
	for index, arg := range os.Args {
		if arg == "events" {
			return os.Args[index+1:]
		}
	}
	return nil
}

func eventFlagValues(args []string) (map[string]string, error) {
	values := map[string]string{}
	for index := 0; index < len(args); index += 2 {
		if !strings.HasPrefix(args[index], "--") || index+1 >= len(args) {
			return nil, fmt.Errorf("flags must use --name value syntax")
		}
		key := strings.TrimPrefix(args[index], "--")
		if values[key] != "" {
			return nil, fmt.Errorf("--%s was provided more than once", key)
		}
		values[key] = strings.TrimSpace(args[index+1])
	}
	return values, nil
}

func eventUsage() {
	fmt.Fprintln(os.Stderr, "usage: mnemo events publish --project PROJECT --directory DIR --agent AGENT --native-id ID --type TYPE --payload VALUE\n       mnemo events invoke --project PROJECT --directory DIR --agent AGENT --native-id ID --tool TOOL --payload JSON\n       mnemo events map [--agent AGENT]")
}
