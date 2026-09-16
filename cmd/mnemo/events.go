package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmeiracorbal/mnemo/adapters"
	"github.com/jmeiracorbal/mnemo/internal/events"
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
		data, err := adapters.EventMapJSON(adapters.Agent(values["agent"]))
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
		event := events.Event{ID: uuid.NewString(), Type: values["type"], Project: project, ExecutionKey: values["execution-key"], Agent: adapters.Agent(values["agent"]), NativeID: values["native-id"], Payload: payload, OccurredAt: time.Now().UTC()}
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
			Agent     adapters.Agent  `json:"agent"`
			NativeID  string          `json:"native_id"`
			Project   string          `json:"project"`
			Directory string          `json:"directory"`
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}{adapters.Agent(values["agent"]), values["native-id"], project, directory, values["tool"], payload}
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
