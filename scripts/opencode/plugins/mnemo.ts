import { Plugin } from "@opencode/plugin"
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

type Entry = { nativeID: string; context: string; injected: boolean }

export default Plugin.define({
  id: "mnemo",
  async setup(ctx) {
    const PROTOCOL = await Bun.file(join(import.meta.dir, "mnemo-protocol.md")).text()
    const directory = ctx.location.directory as string
    const root = gitRoot(directory)
    const project = mnemoProject(root)

    if (!project) return

    const sessions = new Map<string, Entry>()

    function startSession(sessionId: string): Entry {
      try { writeFileSync(sideChannelPath(project!), sessionId) } catch {}
      publish("execution.started", project!, directory, sessionId, { directory })
      const context = invoke("mem_context", project!, directory, sessionId, { project: project! })
      const entry: Entry = { nativeID: sessionId, context, injected: false }
      sessions.set(sessionId, entry)
      return entry
    }

    // Inject memory context before each primary model call.
    // First call for a new sessionID also starts the session in mnemo.
    await ctx.session.hook("context", (event) => {
      const sessionId = event.sessionID as string
      let entry = sessions.get(sessionId)
      if (!entry) {
        entry = startSession(sessionId)
      }
      if (entry.injected) return
      entry.injected = true

      const parts: string[] = [`[mnemo] Session started (project: ${project})`]
      if (entry.context) parts.push(entry.context)
      parts.push(PROTOCOL)

      event.system.push({ type: "text", text: parts.join("\n\n") })
    })

    // On compaction: refresh context and let the next primary call re-inject it.
    await ctx.session.hook("compaction", async (event) => {
      const sessionId = event.sessionID as string
      let entry = sessions.get(sessionId)
      if (!entry) {
        entry = startSession(sessionId)
      }
      const context = invoke("mem_context", project, directory, entry.nativeID, { project })
      if (context) {
        event.system.push({ type: "text", text: context })
        entry.context = context
      }
      entry.injected = false
      publish("session.compacted", project, directory, entry.nativeID, {})
    })
  },
})

