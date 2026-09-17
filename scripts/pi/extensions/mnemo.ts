import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { Type } from "typebox";
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";

type ToolArguments = Record<string, unknown>;

function projectID(cwd: string): string | undefined {
  try {
    const marker = JSON.parse(readFileSync(join(cwd, ".mnemo"), "utf8"));
    return typeof marker.id === "string" && marker.id.trim() ? marker.id.trim() : undefined;
  } catch { return undefined; }
}

// Pi owns the native session ID. This extension is therefore the only mnemo
// tool transport for Pi; a static MCP child cannot receive this per-session ID.
const parameters = Type.Object({
  project: Type.String({ description: "Canonical id from .mnemo" }),
  directory: Type.String({ description: "Current session workspace directory" }),
  query: Type.Optional(Type.String()), type: Type.Optional(Type.String()), scope: Type.Optional(Type.String()),
  title: Type.Optional(Type.String()), content: Type.Optional(Type.String()), topic_key: Type.Optional(Type.String()),
  tags: Type.Optional(Type.String()), prefer_tags: Type.Optional(Type.String()), source: Type.Optional(Type.String()),
  id: Type.Optional(Type.Number()), limit: Type.Optional(Type.Number()), min_count: Type.Optional(Type.Number()),
  max_count: Type.Optional(Type.Number()), unused_since: Type.Optional(Type.String()), sort_by: Type.Optional(Type.String()),
  from: Type.Optional(Type.String()), to: Type.Optional(Type.String()), tag: Type.Optional(Type.String()),
  since: Type.Optional(Type.String()), min_cooccurrence: Type.Optional(Type.Number()),
  include_observations: Type.Optional(Type.Boolean()), include_sessions: Type.Optional(Type.Boolean()),
});

const tools: Array<{ name: string; label: string; description: string }> = [
  { name: "mem_search", label: "Search Memory", description: "Search persistent mnemo memories." },
  { name: "mem_save", label: "Save Memory", description: "Save a structured memory. Requires title and content." },
  { name: "mem_context", label: "Get Memory Context", description: "Read recent context for this project." },
  { name: "mem_session_summary", label: "Save Session Summary", description: "Save the current session summary. Requires content." },
  { name: "mem_get_observation", label: "Get Observation", description: "Read one observation by id." },
  { name: "mem_suggest_topic_key", label: "Suggest Topic Key", description: "Suggest a stable topic key from title or content." },
  { name: "mem_capture_passive", label: "Capture Learnings", description: "Extract learnings from content containing a Key Learnings section." },
  { name: "mem_save_prompt", label: "Save User Prompt", description: "Save a user prompt. Requires content." },
  { name: "mem_update", label: "Update Memory", description: "Update an observation by id." },
  { name: "mem_list_tags", label: "List Tags", description: "List tags for a project." },
  { name: "mem_merge_tags", label: "Merge Tags", description: "Merge one tag into another." },
  { name: "mem_tag_stats", label: "Tag Stats", description: "Inspect tag statistics." },
  { name: "mem_related_tags", label: "Related Tags", description: "Find tags that co-occur with a tag." },
];

type PiEventMapping = { native_type: string; mnemo_type: string; payload_context?: string }

function loadPiEventMap(): PiEventMapping[] {
  try {
    const r = spawnSync("mnemo", ["events", "map", "--agent", "pi"], { encoding: "utf8" })
    if (r.status === 0) return (JSON.parse(r.stdout) as any)?.events ?? []
  } catch {}
  return []
}

export default function (pi: ExtensionAPI) {
  async function publish(type: string, ctx: ExtensionContext, payload: object) {
    const project = projectID(ctx.cwd);
    const nativeID = ctx.sessionManager.getSessionId();
    if (!project || !nativeID) return;
    const result = await pi.exec("mnemo", ["events", "publish", "--project", project, "--directory", ctx.cwd, "--agent", "pi", "--native-id", nativeID, "--type", type, "--payload", JSON.stringify(payload)], { cwd: ctx.cwd });
    if (result.code !== 0) throw new Error(result.stderr.trim() || `mnemo event publish exited ${result.code}`);
  }

  async function invoke(tool: string, args: ToolArguments, signal: AbortSignal | undefined, ctx: ExtensionContext) {
    const project = projectID(ctx.cwd);
    const nativeID = ctx.sessionManager.getSessionId();
    if (!project) throw new Error("mnemo project marker .mnemo with id is required");
    if (!nativeID) throw new Error("Pi did not provide a native session id");
    if (args.project !== project) throw new Error("project must equal the canonical id in .mnemo");
    if (args.directory !== ctx.cwd) throw new Error("directory must equal Pi's current workspace");
    const { project: _project, directory: _directory, ...arguments_ } = args;
    const result = await pi.exec("mnemo", ["events", "invoke", "--project", project, "--directory", ctx.cwd, "--agent", "pi", "--native-id", nativeID, "--tool", tool, "--payload", JSON.stringify(arguments_)], { cwd: ctx.cwd, signal, timeout: 15_000 });
    if (result.code !== 0 || result.stderr.trim()) throw new Error(result.stderr.trim() || `mnemo controller command exited ${result.code}`);
    return result.stdout.trim();
  }

  for (const tool of tools) {
    pi.registerTool({
      ...tool,
      parameters,
      executionMode: "sequential",
      async execute(_toolCallID, args, signal, _onUpdate, ctx) {
        try {
          const text = await invoke(tool.name, args, signal, ctx);
          return { content: [{ type: "text", text }], details: {} };
        } catch (error) {
          return { content: [{ type: "text", text: `mnemo ${tool.name} failed: ${error instanceof Error ? error.message : String(error)}` }], details: {}, isError: true };
        }
      },
    });
  }

  const piEventMap = loadPiEventMap()
  for (const m of piEventMap) {
    pi.on(m.native_type as any, async (_event, ctx) => {
      const payload = m.payload_context === "directory" ? { directory: ctx.cwd } : {}
      await publish(m.mnemo_type, ctx, payload)
    })
  }
}
