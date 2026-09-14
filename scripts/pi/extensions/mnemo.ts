import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

function projectID(cwd: string): string | undefined {
  try {
    const marker = JSON.parse(readFileSync(join(cwd, ".mnemo"), "utf8"));
    return typeof marker.id === "string" && marker.id.trim() ? marker.id.trim() : undefined;
  } catch { return undefined; }
}

export default function (pi: ExtensionAPI) {
  async function publish(type: string, ctx: any, payload: object) {
    const project = projectID(ctx.cwd);
    const nativeID = ctx.sessionManager.getSessionId();
    if (!project || !nativeID) return;
    await pi.exec("mnemo", ["events", "publish", "--project", project, "--directory", ctx.cwd, "--agent", "pi", "--native-id", nativeID, "--type", type, "--payload", JSON.stringify(payload)]);
  }
  pi.on("session_start", async (_event, ctx) => publish("execution.started", ctx, { directory: ctx.cwd }));
  pi.on("session_shutdown", async (_event, ctx) => publish("execution.closed", ctx, {}));
  pi.on("session_compact", async (_event, ctx) => publish("session.compacted", ctx, {}));
  pi.on("tool_result", async (event, ctx) => publish("agent.tool_result", ctx, { tool: event.toolName }));
}
