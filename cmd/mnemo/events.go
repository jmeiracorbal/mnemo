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
		fmt.Fprintln(os.Stderr, "mnemo events:", err)
		return
	}
	project, directory := values["project"], values["directory"]
	if project == "" || directory == "" {
		fmt.Fprintln(os.Stderr, "mnemo events: --project and --directory are required")
		return
	}
	storeConfig, err := store.DefaultConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mnemo events:", err)
		return
	}
	cfg, err := events.LoadConfig(storeConfig.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mnemo events:", err)
		return
	}

	switch command {
	case "publish":
		payload := json.RawMessage(values["payload"])
		event := events.Event{ID: uuid.NewString(), Type: values["type"], Project: project, ExecutionKey: values["execution-key"], Agent: adapters.Agent(values["agent"]), NativeID: values["native-id"], Payload: payload, OccurredAt: time.Now().UTC()}
		if err := event.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "mnemo events:", err)
			return
		}
		if err := events.Publish(context.Background(), cfg, event); err != nil {
			fmt.Fprintln(os.Stderr, "mnemo events:", err)
		}
	default:
		eventUsage()
	}
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
	fmt.Fprintln(os.Stderr, "usage: mnemo events publish --project PROJECT --directory DIR --execution-key KEY --type TYPE --payload VALUE")
}
