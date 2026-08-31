package main

import (
	_ "embed"
	"fmt"
	"os"
	"text/template"

	"github.com/jmeiracorbal/mnemo/internal/store"
)

var version = "dev"

//go:embed templates/usage.txt
var usageTemplateText string

var usageTemplate = template.Must(template.New("usage").Parse(usageTemplateText))

type usageTemplateData struct {
	Version string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	maybeWarnUpdate(os.Args[1:])

	// Commands that don't need the store.
	switch os.Args[1] {
	case "json":
		runJSON()
		return
	case "json-merge":
		runJSONMerge()
		return
	case "extract-transcript":
		runExtractTranscript()
		return
	case "install-instructions":
		runInstallInstructions()
		return
	case "doctor":
		runDoctor()
		return
	case "db":
		runDB()
		return
	case "update":
		runUpdate()
		return
	case "setup":
		runSetup()
		return
	case "--version", "version":
		fmt.Printf("mnemo %s\n", version)
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

	switch os.Args[1] {
	case "init":
		runInit(s)
	case "projects":
		runProjects(s)
	case "memories":
		runMemories(s)
	case "migrate":
		runMigrateProjects(s)
	case "mcp":
		runMCP(s)
	case "save":
		runSave(s)
	case "search":
		runSearch(s)
	case "context":
		runContext(s)
	case "session":
		runSession(s)
	case "stats":
		runStats(s)
	case "export":
		runExport(s)
	case "import":
		runImport(s)
	case "capture":
		runCapture(s)
	default:
		fmt.Fprintf(os.Stderr, "mnemo: unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	if err := usageTemplate.Execute(os.Stderr, usageTemplateData{Version: version}); err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: render usage: %v\n", err)
	}
}
