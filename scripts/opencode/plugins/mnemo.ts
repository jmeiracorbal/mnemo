import type { Plugin } from "@opencode-ai/plugin"
import { existsSync, readFileSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"

function gitRoot(cwd: string): string {
  const r = Bun.spawnSync(["git", "-C", cwd, "rev-parse", "--show-toplevel"], { stderr: "ignore" })
  return r.exitCode === 0 ? (r.stdout?.toString().trim() ?? cwd) : cwd
}

function mnemoProject(root: string): string | null {
  const marker = `${root}/.mnemo`
  if (!existsSync(marker)) return null
  try {
    return (JSON.parse(readFileSync(marker, "utf8")) as { id?: string }).id ?? null
  } catch {
    return null
  }
}

function sideChannelPath(project: string): string {
  return join(tmpdir(), `mnemo-opencode-${project}`)
}

function run(cmd: string[]): { ok: boolean; out: string } {
  const r = Bun.spawnSync(cmd, { stderr: "ignore" })
  return { ok: r.exitCode === 0, out: r.stdout?.toString().trim() ?? "" }
}

function publish(type: string, project: string, directory: string, nativeID: string, payload: object): void {
  run(["mnemo", "events", "publish",
    "--project", project, "--directory", directory,
    "--agent", "opencode", "--native-id", nativeID,
    "--type", type, "--payload", JSON.stringify(payload),
  ])
}

function invoke(tool: string, project: string, directory: string, nativeID: string, args: Record<string, unknown>): string {
  const r = run(["mnemo", "events", "invoke",
    "--project", project, "--directory", directory,
    "--agent", "opencode", "--native-id", nativeID,
    "--tool", tool, "--payload", JSON.stringify(args),
  ])
  return r.ok ? r.out : ""
}

type EventMapping = { native_type: string; mnemo_type: string; native_id_path: string; payload_context?: string; lifecycle_action?: string }

// Plugin-level lifecycle action labels. Values must match adapters.LifecycleAction* constants in Go.
const LIFECYCLE_START_SESSION = "start_session"

function loadEventMap(agent: string): EventMapping[] {
  const r = Bun.spawnSync(["mnemo", "events", "map", "--agent", agent], { stderr: "ignore" })
  if (r.exitCode !== 0) return []
  try {
    return (JSON.parse(r.stdout?.toString() ?? "{}") as any)?.events ?? []
  } catch {
    return []
  }
}

function resolvePropertyPath(obj: unknown, path: string): string | undefined {
  if (!path) return undefined
  let current: unknown = obj
  for (const part of path.split(".")) {
    if (current == null || typeof current !== "object") return undefined
    current = (current as Record<string, unknown>)[part]
  }
  return typeof current === "string" ? current : undefined
}

type Entry = { nativeID: string; context: string; injected: boolean }

export const Mnemo: Plugin = async (ctx) => {
  const PROTOCOL = await Bun.file(`${import.meta.dir}/mnemo-protocol.md`).text()
  const root = gitRoot(ctx.directory)
  const project = mnemoProject(root)
  const sessions = new Map<string, Entry>()
  const eventMap = loadEventMap("opencode")

  function startSession(sessionId: string): Entry {
    try { writeFileSync(sideChannelPath(project!), sessionId) } catch {}
    publish("execution.started", project!, ctx.directory, sessionId, { directory: ctx.directory })
    const context = invoke("mem_context", project!, ctx.directory, sessionId, { project: project! })
    const entry: Entry = { nativeID: sessionId, context, injected: false }
    sessions.set(sessionId, entry)
    return entry
  }

  return {
    event: async ({ event }) => {
      if (!project) return
      const mapping = eventMap.find(m => m.native_type === event.type)
      if (!mapping) return
      const sessionId = resolvePropertyPath(event.properties, mapping.native_id_path)
      if (!sessionId) return
      if (mapping.lifecycle_action === LIFECYCLE_START_SESSION) {
        if (!sessions.has(sessionId)) startSession(sessionId)
        return
      }
      const entry = sessions.get(sessionId)
      if (!entry) return
      publish(mapping.mnemo_type, project, ctx.directory, entry.nativeID, {})
    },

    "experimental.chat.system.transform": async (input, output) => {
      if (!project || !input.sessionID) return
      let entry = sessions.get(input.sessionID)
      if (!entry) {
        // Resume: session started before the plugin loaded.
        entry = startSession(input.sessionID)
      }
      if (entry.injected) return
      entry.injected = true

      const parts: string[] = [`[mnemo] Session started (project: ${project})`]
      if (entry.context) parts.push(entry.context)
      parts.push(PROTOCOL)

      const msg = parts.join("\n\n")
      if (output.system.length > 0) {
        output.system[output.system.length - 1] += "\n\n" + msg
      } else {
        output.system.push(msg)
      }
    },

    "experimental.session.compacting": async (input, output) => {
      const entry = sessions.get(input.sessionID)
      if (!entry || !project) return
      const context = invoke("mem_context", project, ctx.directory, entry.nativeID, { project })
      if (context) {
        output.context.push(context)
        entry.context = context
      }
      entry.injected = false
    },
  }
}
