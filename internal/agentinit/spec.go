package agentinit

import (
	"fmt"
	"strings"
)

// AgentSpec describes the mnemo integration surfaces owned by one agent.
type AgentSpec struct {
	ID           string
	Label        string
	Detect       func(home string) bool
	MCP          MCPConfigSpec
	Instructions []InstructionSpec
	Hooks        []HookSpec
	Supports     AgentSpecCapabilities
}

// AgentSpecCapabilities records which integration surfaces an AgentSpec supports.
type AgentSpecCapabilities struct {
	MCP              bool
	MCPConditional   bool
	Instructions     bool
	Skills           bool
	Hooks            bool
	SessionLifecycle bool
	RPC              bool
	ACP              bool
}

// MCPConfigSpec describes an agent MCP configuration surface.
type MCPConfigSpec struct {
	Snippets  func(home, mnemoBin string) []ConfigSnippet
	Uninstall func(home string) ([]string, error)
	Check     func(home string) Check
}

// InstructionScope identifies where an instruction surface is installed.
type InstructionScope string

const (
	InstructionScopeGlobal  InstructionScope = "global"
	InstructionScopeProject InstructionScope = "project"
)

// InstructionSpec describes an agent instruction surface.
type InstructionSpec struct {
	Scope   InstructionScope
	Path    func(home string) string
	Install func(home string) (string, error)
	Remove  func(home string) (string, bool, error)
	Check   func(home string) Check
}

// HookSpec describes agent runtime assets or hook/plugin validation.
type HookSpec struct {
	RuntimeAssets func() []assetTarget
	Check         func(home string) Check
}

var agentSpecs = []AgentSpec{
	{
		ID:     AgentClaudeCode,
		Label:  claudeCodeLabel(),
		Detect: detectFromPaths(claudeCodeDetectionPaths),
		MCP: MCPConfigSpec{
			Snippets:  claudeCodeConfigSnippets,
			Uninstall: claudeCodeUninstallConfig,
			Check:     claudeCodeCheckMCP,
		},
		Instructions: []InstructionSpec{{
			Scope:   InstructionScopeGlobal,
			Path:    claudeCodeInstructionPath,
			Install: claudeCodeInstallInstructions,
			Remove:  claudeCodeRemoveInstructions,
			Check:   claudeCodeCheckInstructions,
		}, {
			Scope:   InstructionScopeProject,
			Path:    claudeCodeProjectInstructionPath,
			Install: claudeCodeInstallProjectInstructions,
		}},
		Hooks: []HookSpec{{
			RuntimeAssets: claudeCodeRuntimeAssets,
			Check:         claudeCodeCheckRuntime,
		}},
		Supports: AgentSpecCapabilities{MCP: true, Instructions: true, Skills: true, Hooks: true},
	},
	{
		ID:     AgentCursor,
		Label:  cursorLabel(),
		Detect: detectFromPaths(cursorDetectionPaths),
		MCP: MCPConfigSpec{
			Snippets:  cursorConfigSnippets,
			Uninstall: cursorUninstallConfig,
			Check:     cursorCheckMCP,
		},
		Instructions: []InstructionSpec{{
			Scope:   InstructionScopeGlobal,
			Path:    cursorInstructionPath,
			Install: cursorInstallInstructions,
			Remove:  cursorRemoveInstructions,
			Check:   cursorCheckInstructions,
		}, {
			Scope:   InstructionScopeProject,
			Path:    cursorProjectInstructionPath,
			Install: cursorInstallProjectInstructions,
		}},
		Hooks: []HookSpec{{
			RuntimeAssets: cursorRuntimeAssets,
			Check:         cursorCheckRuntime,
		}},
		Supports: AgentSpecCapabilities{MCP: true, Instructions: true, Hooks: true, SessionLifecycle: true},
	},
	{
		ID:     AgentWindsurf,
		Label:  windsurfLabel(),
		Detect: detectFromPaths(windsurfDetectionPaths),
		MCP: MCPConfigSpec{
			Snippets:  windsurfConfigSnippets,
			Uninstall: windsurfUninstallConfig,
			Check:     windsurfCheckMCP,
		},
		Instructions: []InstructionSpec{{
			Scope:   InstructionScopeGlobal,
			Path:    windsurfInstructionPath,
			Install: windsurfInstallInstructions,
			Remove:  windsurfRemoveInstructions,
			Check:   windsurfCheckInstructions,
		}, {
			Scope:   InstructionScopeProject,
			Path:    windsurfProjectInstructionPath,
			Install: windsurfInstallProjectInstructions,
		}},
		Hooks: []HookSpec{{
			RuntimeAssets: windsurfRuntimeAssets,
			Check:         windsurfCheckRuntime,
		}},
		Supports: AgentSpecCapabilities{MCP: true, Instructions: true, Skills: true, Hooks: true, SessionLifecycle: true},
	},
	{
		ID:     AgentCodex,
		Label:  codexLabel(),
		Detect: detectFromPaths(codexDetectionPaths),
		MCP: MCPConfigSpec{
			Snippets:  codexConfigSnippets,
			Uninstall: codexUninstallConfig,
			Check:     codexCheckMCP,
		},
		Instructions: []InstructionSpec{{
			Scope:   InstructionScopeGlobal,
			Path:    codexInstructionPath,
			Install: codexInstallInstructions,
			Remove:  codexRemoveInstructions,
			Check:   codexCheckInstructions,
		}},
		Hooks: []HookSpec{{
			RuntimeAssets: codexRuntimeAssets,
			Check:         codexCheckRuntime,
		}},
		Supports: AgentSpecCapabilities{MCP: true, Instructions: true, Hooks: true, SessionLifecycle: true},
	},
	{
		ID:     AgentOpenCode,
		Label:  openCodeLabel(),
		Detect: detectFromPaths(openCodeDetectionPaths),
		MCP: MCPConfigSpec{
			Snippets:  openCodeConfigSnippets,
			Uninstall: openCodeUninstallConfig,
			Check:     openCodeCheckMCP,
		},
		Instructions: []InstructionSpec{{
			Scope:   InstructionScopeGlobal,
			Path:    openCodeInstructionPath,
			Install: openCodeInstallInstructions,
			Remove:  openCodeRemoveInstructions,
			Check:   openCodeCheckInstructions,
		}},
		Hooks: []HookSpec{{
			RuntimeAssets: openCodeRuntimeAssets,
			Check:         openCodeCheckRuntime,
		}},
		Supports: AgentSpecCapabilities{MCP: true, Instructions: true, Hooks: true},
	},
	{
		ID:     AgentFx,
		Label:  fxLabel(),
		Detect: detectFromPaths(fxDetectionPaths),
		MCP: MCPConfigSpec{
			Snippets:  fxConfigSnippets,
			Uninstall: fxUninstallConfig,
			Check:     fxCheckMCP,
		},
		Instructions: []InstructionSpec{{
			Scope:   InstructionScopeGlobal,
			Path:    fxInstructionPath,
			Install: fxInstallInstructions,
			Remove:  fxRemoveInstructions,
			Check:   fxCheckInstructions,
		}},
		Hooks: []HookSpec{{
			RuntimeAssets: fxRuntimeAssets,
			Check:         fxCheckRuntime,
		}},
		Supports: AgentSpecCapabilities{MCP: true, Instructions: true, Skills: true},
	},
}

var agentSpecsByID = indexAgentSpecs(agentSpecs)

// Spec returns the AgentSpec for agent.
func Spec(agent string) (AgentSpec, bool) {
	spec, ok := agentSpecsByID[agent]
	return spec, ok
}

func lookupAgentSpec(agent string) (AgentSpec, error) {
	spec, ok := Spec(agent)
	if !ok {
		return AgentSpec{}, fmt.Errorf("unknown agent %q", agent)
	}
	return spec, nil
}

func globalInstructionSpec(spec AgentSpec) (InstructionSpec, bool) {
	for _, instruction := range spec.Instructions {
		if instruction.Scope == InstructionScopeGlobal {
			return instruction, true
		}
	}
	return InstructionSpec{}, false
}

func projectInstructionSpecs(spec AgentSpec) []InstructionSpec {
	var instructions []InstructionSpec
	for _, instruction := range spec.Instructions {
		if instruction.Scope == InstructionScopeProject {
			instructions = append(instructions, instruction)
		}
	}
	return instructions
}

func agentSpecIDs(specs []AgentSpec) []string {
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		ids = append(ids, spec.ID)
	}
	return ids
}

func indexAgentSpecs(specs []AgentSpec) map[string]AgentSpec {
	indexed := make(map[string]AgentSpec, len(specs))
	for _, spec := range specs {
		indexed[spec.ID] = spec
	}
	return indexed
}

func validAgentList() string {
	return strings.Join(append(append([]string(nil), SupportedAgents...), "all"), " | ")
}

func detectFromPaths(paths func(home string) []string) func(home string) bool {
	return func(home string) bool {
		for _, path := range paths(home) {
			if pathExists(path) {
				return true
			}
		}
		return false
	}
}
