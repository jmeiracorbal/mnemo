package main

import (
	"fmt"
	"os"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/spf13/cobra"
)

// newRootCommand defines the CLI menu in code. Leaf commands deliberately keep
// the existing argument parsers until they can be migrated independently; the
// command tree owns discovery and help without changing their contracts.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "mnemo",
		Short:         "Persistent memory for AI coding agents",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return fmt.Errorf("a command is required")
		},
	}
	root.Version = version
	root.SetVersionTemplate("mnemo {{.Version}}\n")

	root.AddCommand(
		storeCommand("save <title> <content> [--type=TYPE] [--project=PROJECT] [--scope=SCOPE] [--topic=TOPIC]", "Save a memory", runSave),
		storeCommand("search <query> [--project=PROJECT] [--scope=SCOPE] [--limit=N]", "Search memories", runSearch),
		storeCommand("context [project]", "Show context from previous sessions", runContext),
		storeCommand("stats", "Show memory statistics", runStats),
		storeCommand("export [file]", "Export all memories to JSON", runExport),
		storeCommand("import <file.json>", "Import memories from JSON", runImport),
		storeCommand("capture <content>|-", "Extract learnings from text", runCapture),
		storeCommand("init [--agent=AGENT] [--path=DIR] [--no-project-rules]", "Activate mnemo in the current project", runInit),
		storeCommand("migrate", "Migrate project identity", runMigrateProjects),
		storeCommand("mcp", "Start MCP server (stdio)", runMCP),
		storeCommand("projects", "Manage known projects", runProjects),
		storeCommand("memories", "Review and curate memories", runMemories),
		storeCommand("session", "Manage memory sessions", runSession),
		newSyncCommand(),
		command("json [KEY ...]", "Extract fields from JSON on stdin", runJSON),
		command("json-merge <file>", "Deep-merge JSON from stdin into a file", runJSONMerge),
		command("extract-transcript <file>", "Extract assistant text from a JSONL transcript", runExtractTranscript),
		command("install-instructions [--agent=AGENT]", "Install global agent instructions", runInstallInstructions),
		command("doctor [--json] [--agent=AGENT] [--path=DIR] [--home=DIR] [--data-dir=DIR]", "Run read-only diagnostics", runDoctor),
		command("db", "Validate or apply database migrations", runDB),
		command("update [--check] [--yes] [--agent=AGENT] [--json]", "Check for and install a newer mnemo release", runUpdate),
		command("setup", "Manage global agent setup", runSetup),
		&cobra.Command{Use: "version", Short: "Show version", Run: func(*cobra.Command, []string) { fmt.Printf("mnemo %s\n", version) }},
	)

	addCommandTree(root)
	return root
}

func command(use, short string, run func()) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		Short:              short,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if hasHelpArg(args) {
				_ = cmd.Help()
				return
			}
			run()
		},
	}
}

func storeCommand(use, short string, run func(*store.Store)) *cobra.Command {
	cmd := command(use, short, func() {})
	cmd.Run = func(cmd *cobra.Command, args []string) {
		if hasHelpArg(args) {
			_ = cmd.Help()
			return
		}
		cfg, err := store.DefaultConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: config error: %v\n", err)
			os.Exit(1)
		}
		s, err := store.New(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mnemo: failed to open store: %v\n", err)
			os.Exit(1)
		}
		defer s.Close()
		run(s)
	}
	return cmd
}

func hasHelpArg(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func addCommandTree(root *cobra.Command) {
	addChildren(root, "db", command("migrate [--data-dir=DIR] [--check] [--json]", "Validate or apply database migrations", runDBMigrate))
	addChildren(root, "setup",
		command("status [--json] [--agent=AGENT] [--home=DIR]", "Show global agent setup status", runSetupStatus),
		command("print-config AGENT [--home=DIR] [--mnemo-bin=PATH]", "Print manual setup config snippets", runSetupPrintConfig),
		command("refresh [--agent=AGENT] [--home=DIR] [--mnemo-bin=PATH]", "Refresh installed global setup files", runSetupRefresh),
		command("uninstall --agent=AGENT [--home=DIR]", "Remove global setup files", runSetupUninstall),
		command("cloud [--non-interactive] [--validate] [--delete] [--url=URL] [--key=KEY] [--client-id=ID] [--provider=PROVIDER]", "Configure cloud sync credentials", runSetupCloud),
	)
	for _, agent := range append(agentinit.SupportedAgents, "all") {
		addChildren(root, "setup", command(agent, "Alias for setup refresh", runSetup))
	}
	addChildren(root, "projects",
		storeCommand("list [--sort=FIELD] [--asc|--desc] [--unused-since=AGE] [--empty] [--json]", "List known projects", runProjectsList),
		storeCommand("merge --from=PROJECT --to=PROJECT (--dry-run|--yes) [--json]", "Merge duplicate project identities", runProjectsMerge),
		storeCommand("rename (--id=PROJECT|--path=DIR) --name=NAME (--dry-run|--yes) [--json]", "Rename project display metadata", runProjectsRename),
	)
	addChildren(root, "memories",
		storeCommand("review [--project=PROJECT] [--scope=SCOPE] [--topic=TOPIC_KEY] [--limit=N] [--json]", "Review potential memory conflicts", runMemoriesReview),
		storeCommand("mark-reviewed OBSERVATION_ID [--reason=TEXT]", "Mark a memory as reviewed", runMemoriesMarkReviewed),
		storeCommand("mark-stale OBSERVATION_ID [--reason=TEXT]", "Mark a memory as stale", runMemoriesMarkStale),
		storeCommand("supersede OLD_ID --by=NEW_ID [--reason=TEXT]", "Mark a memory as superseded", runMemoriesSupersede),
		storeCommand("consolidate-topic --from=TOPIC --to=TOPIC (--dry-run|--yes) [--project=PROJECT] [--scope=SCOPE] [--json]", "Consolidate memory topic keys", runMemoriesConsolidateTopic),
	)
	addChildren(root, "session",
		storeCommand("start ID", "Register session start", runSession),
		storeCommand("end ID", "Mark session completed", runSession),
		storeCommand("exists ID", "Check whether a session exists", runSession),
		storeCommand("obs-count ID", "Count session observations", runSession),
		storeCommand("project-obs-count ID", "Count project observations", runSession),
	)
}

func addChildren(root *cobra.Command, parentUse string, children ...*cobra.Command) {
	var parent *cobra.Command
	for _, candidate := range root.Commands() {
		if candidate.Name() == parentUse {
			parent = candidate
			break
		}
	}
	if parent != nil {
		parent.AddCommand(children...)
	}
}
