package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

type setupPrintConfigOptions struct {
	Agent    string
	Home     string
	MnemoBin string
}

type setupConfigSnippet struct {
	Agent   string
	Path    string
	Format  string
	Content string
}

func runSetupPrintConfig() {
	opts, err := parseSetupPrintConfigArgs(os.Args[3:], os.UserHomeDir, exec.LookPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup print-config: %v\n", err)
		os.Exit(1)
	}
	snippets, err := buildSetupConfigSnippets(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo setup print-config: %v\n", err)
		os.Exit(1)
	}
	printSetupConfigSnippets(snippets)
}

func parseSetupPrintConfigArgs(args []string, userHomeDir func() (string, error), lookPath func(string) (string, error)) (setupPrintConfigOptions, error) {
	opts := setupPrintConfigOptions{}
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--home="):
			opts.Home = arg[len("--home="):]
		case strings.HasPrefix(arg, "--mnemo-bin="):
			opts.MnemoBin = arg[len("--mnemo-bin="):]
		case strings.HasPrefix(arg, "--"):
			return opts, fmt.Errorf("unknown option %q", arg)
		case opts.Agent == "":
			opts.Agent = strings.TrimSpace(arg)
		default:
			return opts, fmt.Errorf("unexpected argument %q", arg)
		}
	}
	if opts.Agent == "" {
		return opts, fmt.Errorf("missing agent — usage: mnemo setup print-config AGENT")
	}
	if opts.Home == "" {
		home, err := userHomeDir()
		if err != nil {
			return opts, fmt.Errorf("home: %w", err)
		}
		opts.Home = home
	}
	if opts.MnemoBin == "" {
		if path, err := lookPath("mnemo"); err == nil && strings.TrimSpace(path) != "" {
			opts.MnemoBin = path
		} else {
			opts.MnemoBin = "mnemo"
		}
	}
	return opts, nil
}

func buildSetupConfigSnippets(opts setupPrintConfigOptions) ([]setupConfigSnippet, error) {
	agents, err := agentinit.ExpandAgents(opts.Agent)
	if err != nil {
		return nil, err
	}
	var snippets []setupConfigSnippet
	for _, agent := range agents {
		agentSnippets, err := setupConfigSnippetsForAgent(opts.Home, opts.MnemoBin, agent)
		if err != nil {
			return nil, err
		}
		snippets = append(snippets, agentSnippets...)
	}
	return snippets, nil
}

func setupConfigSnippetsForAgent(home, mnemoBin, agent string) ([]setupConfigSnippet, error) {
	switch agent {
	case "claudecode":
		return []setupConfigSnippet{{
			Agent:   agentLabel(agent),
			Path:    filepath.Join(home, ".claude", ".mcp.json"),
			Format:  "json",
			Content: mcpServersJSON(mnemoBin),
		}}, nil
	case "cursor":
		hooksDir := filepath.Join(home, ".cursor", "hooks")
		return []setupConfigSnippet{
			{
				Agent:   agentLabel(agent),
				Path:    filepath.Join(home, ".cursor", "mcp.json"),
				Format:  "json",
				Content: mcpServersJSON(mnemoBin),
			},
			{
				Agent:  agentLabel(agent),
				Path:   filepath.Join(home, ".cursor", "hooks.json"),
				Format: "json",
				Content: prettyJSON(map[string]any{
					"version": 1,
					"hooks": map[string]any{
						"beforeSubmitPrompt": []any{map[string]any{"command": filepath.Join(hooksDir, "before-submit-prompt.sh")}},
						"stop":               []any{map[string]any{"command": filepath.Join(hooksDir, "stop.sh")}},
					},
				}),
			},
		}, nil
	case "windsurf":
		hooksDir := filepath.Join(home, ".codeium", "windsurf", "hooks")
		return []setupConfigSnippet{
			{
				Agent:   agentLabel(agent),
				Path:    filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"),
				Format:  "json",
				Content: mcpServersJSON(mnemoBin),
			},
			{
				Agent:  agentLabel(agent),
				Path:   filepath.Join(home, ".codeium", "windsurf", "hooks.json"),
				Format: "json",
				Content: prettyJSON(map[string]any{
					"hooks": map[string]any{
						"pre_user_prompt":                       []any{map[string]any{"command": filepath.Join(hooksDir, "pre-user-prompt.sh")}},
						"post_cascade_response_with_transcript": []any{map[string]any{"command": filepath.Join(hooksDir, "post-cascade-response.sh")}},
					},
				}),
			},
		}, nil
	case "codex":
		hooksDir := filepath.Join(home, ".codex", "hooks")
		return []setupConfigSnippet{
			{
				Agent:   agentLabel(agent),
				Path:    filepath.Join(home, ".codex", "config.toml"),
				Format:  "toml",
				Content: fmt.Sprintf("[mcp_servers.mnemo]\ncommand = %q\nargs = [\"mcp\", \"--tools=agent\"]\n", mnemoBin),
			},
			{
				Agent:  agentLabel(agent),
				Path:   filepath.Join(home, ".codex", "hooks.json"),
				Format: "json",
				Content: prettyJSON(map[string]any{
					"hooks": map[string]any{
						"SessionStart": []any{map[string]any{
							"matcher": "startup|resume",
							"hooks": []any{map[string]any{
								"type":          "command",
								"command":       filepath.Join(hooksDir, "session-start.sh"),
								"statusMessage": "Loading mnemo memory...",
								"timeout":       10,
							}},
						}},
						"Stop": []any{map[string]any{
							"matcher": "",
							"hooks": []any{map[string]any{
								"type":    "command",
								"command": filepath.Join(hooksDir, "stop.sh"),
								"timeout": 10,
							}},
						}},
					},
				}),
			},
		}, nil
	case "opencode":
		return []setupConfigSnippet{{
			Agent:  agentLabel(agent),
			Path:   filepath.Join(home, ".config", "opencode", "opencode.json"),
			Format: "json",
			Content: prettyJSON(map[string]any{
				"mcp": map[string]any{
					"mnemo": map[string]any{
						"type":    "local",
						"command": []string{mnemoBin, "mcp", "--tools=agent"},
					},
				},
			}),
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported agent %q for setup print-config", agent)
	}
}

func mcpServersJSON(mnemoBin string) string {
	return prettyJSON(map[string]any{
		"mcpServers": map[string]any{
			"mnemo": map[string]any{
				"command": mnemoBin,
				"args":    []string{"mcp", "--tools=agent"},
			},
		},
	})
}

func prettyJSON(v any) string {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(out) + "\n"
}

func printSetupConfigSnippets(snippets []setupConfigSnippet) {
	for i, snippet := range snippets {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("# %s: %s (%s)\n", snippet.Agent, snippet.Path, snippet.Format)
		fmt.Print(snippet.Content)
	}
}
