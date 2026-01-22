import { spawn } from "node:child_process";
import os from "node:os";

/**
 * tasker plugin tool for Clawdbot.
 *
 * Registers OPTIONAL tool `tasker_cmd` which spawns the `tasker` CLI safely:
 * - shell: false
 * - argv array
 *
 * Designed to be used by a skill with `command-dispatch: tool`.
 */

type PluginApi = any;

const ToolParams = {
  type: "object",
  additionalProperties: false,
  properties: {
    command: { type: "string", description: "Raw args after /task" },
    commandName: { type: "string" },
    skillName: { type: "string" },
  },
  required: ["command"],
} as const;

function splitArgs(input: string): string[] {
  const out: string[] = [];
  let i = 0;
  const n = input.length;
  const isWs = (c: string) => c === " " || c === "\t" || c === "\n" || c === "\r";

  while (i < n) {
    while (i < n && isWs(input[i]!)) i++;
    if (i >= n) break;

    let token = "";
    let quote: '"' | "'" | null = null;

    while (i < n) {
      const c = input[i]!;
      if (!quote && isWs(c)) break;

      if (!quote && (c === '"' || c === "'")) {
        quote = c as any;
        i++;
        continue;
      }

      if (quote && c === quote) {
        quote = null;
        i++;
        continue;
      }

      if (c === "\\" && i + 1 < n) {
        const next = input[i + 1]!;
        if (next === "\\" || next === '"' || next === "'") {
          token += next;
          i += 2;
          continue;
        }
      }

      token += c;
      i++;
    }

    if (quote) throw new Error("Unmatched quote in command string");
    out.push(token);
    while (i < n && isWs(input[i]!)) i++;
  }

  return out;
}

function isMutating(argv: string[]): boolean {
  const verb = argv[0] ?? "";
  if (!verb) return false;

  // Task mutations
  if (["add", "edit", "done", "mv", "move", "rm", "init"].includes(verb)) return true;
  if (verb === "note" && argv[1] === "add") return true;

  // Project mutations
  if (verb === "project" && argv[1] === "add") return true;

  return false;
}

function defaultRoot(): string {
  const home = os.homedir();
  return home ? `${home}/.tasker` : ".tasker";
}

function runTasker(opts: { binary: string; rootPath: string; timeoutMs: number; argv: string[] }) {
  return new Promise<{ stdout: string; stderr: string; code: number | null }>((resolve) => {
    const args: string[] = ["--root", opts.rootPath, ...opts.argv];

    let stdout = "";
    let stderr = "";
    let settled = false;

    const finish = (result: { stdout: string; stderr: string; code: number | null }) => {
      if (settled) return;
      settled = true;
      resolve(result);
    };

    const child = spawn(opts.binary, args, {
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
    });

    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");

    child.stdout.on("data", (d) => (stdout += d));
    child.stderr.on("data", (d) => (stderr += d));

    const to = setTimeout(() => {
      try { child.kill("SIGKILL"); } catch {}
    }, opts.timeoutMs);

    child.on("error", (err: any) => {
      clearTimeout(to);
      const msg =
        err?.code === "ENOENT"
          ? `tasker_cmd: binary not found (${opts.binary}). Set plugin config "binary" or add it to PATH.`
          : `tasker_cmd: ${err?.message ?? String(err)}`;
      finish({ stdout, stderr: msg, code: 127 });
    });

    child.on("close", (code) => {
      clearTimeout(to);
      finish({ stdout, stderr, code });
    });
  });
}

export default function (api: PluginApi) {
  api.registerTool(
    {
      name: "tasker_cmd",
      description: "Run tasker CLI subcommands safely (no shell). Used by /task skill tool-dispatch.",
      parameters: ToolParams,
      async execute(_id: string, params: any) {
        const cfg = (api.pluginConfig ?? {}) as {
          binary?: string;
          rootPath?: string;
          timeoutMs?: number;
          allowWrite?: boolean;
        };
        const envBinary = process.env.TASKER_BIN ? String(process.env.TASKER_BIN).trim() : "";
        const binary = (cfg.binary && String(cfg.binary)) || envBinary || "tasker";
        const rootPath = (cfg.rootPath && String(cfg.rootPath)) || defaultRoot();
        const timeoutMs = cfg.timeoutMs ? Number(cfg.timeoutMs) : 15000;
        const allowWrite = cfg.allowWrite !== undefined ? Boolean(cfg.allowWrite) : true;

        let argv: string[];
        try {
          argv = splitArgs(String(params.command || ""));
        } catch (e: any) {
          return { content: [{ type: "text", text: `tasker_cmd: invalid command string: ${e?.message ?? String(e)}` }] };
        }

        if (!argv.length) {
          return { content: [{ type: "text", text: "Usage: /task <subcommand> [args]. Example: /task ls --project Work" }] };
        }

        if (!allowWrite && isMutating(argv)) {
          return { content: [{ type: "text", text: "tasker_cmd: write operations are disabled (allowWrite=false)." }] };
        }

        const { stdout, stderr, code } = await runTasker({ binary, rootPath, timeoutMs, argv });

        const parts: string[] = [];
        if (stdout.trim()) parts.push(stdout.trimEnd());
        if (stderr.trim()) parts.push(`\n[stderr]\n${stderr.trimEnd()}`);
        if (code && code !== 0) parts.push(`\n[exit] ${code}`);

        return { content: [{ type: "text", text: parts.join("\n") || "(no output)" }] };
      },
    },
    { optional: true },
  );
}
