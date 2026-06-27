import type { Plugin } from "@opencode-ai/plugin"
import { existsSync, readFileSync } from "node:fs"

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

function run(cmd: string[]): { ok: boolean; out: string } {
  const r = Bun.spawnSync(cmd, { stderr: "ignore" })
  return { ok: r.exitCode === 0, out: r.stdout?.toString().trim() ?? "" }
}

type Entry = { context: string; injected: boolean }

export const Mnemo: Plugin = async (ctx) => {
  const PROTOCOL = await Bun.file(`${import.meta.dir}/mnemo-protocol.md`).text()
  const root = gitRoot(ctx.directory)
  const project = mnemoProject(root)
  const sessions = new Map<string, Entry>()

  return {
    event: async ({ event }) => {
      if (event.type !== "session.created" || !project) return
      const sessionId: string = (event.properties as any)?.info?.id
      if (!sessionId || sessions.has(sessionId)) return

      run(["mnemo", "session", "start", sessionId, "--project", project, "--dir", ctx.directory])
      const c = run(["mnemo", "context", project])
      sessions.set(sessionId, { context: c.ok ? c.out : "", injected: false })
    },

    "experimental.chat.system.transform": async (input, output) => {
      if (!project) return
      let entry = sessions.get(input.sessionID)
      if (!entry) {
        // Session started before the plugin loaded (resume scenario)
        const c = run(["mnemo", "context", project])
        entry = { context: c.ok ? c.out : "", injected: false }
        sessions.set(input.sessionID, entry)
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
      const r = run(["mnemo", "context", project])
      if (r.ok && r.out) {
        output.context.push(r.out)
        entry.context = r.out
      }
      entry.injected = false
    },
  }
}
