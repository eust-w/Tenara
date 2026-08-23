import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { spawn } from "node:child_process";
import test from "node:test";

import { ADMIN_TOOLS } from "../src/tools_admin.ts";
import { APP_TOOLS } from "../src/tools.ts";

const EXPECTED_NAMES = [
  "ping",
  ...APP_TOOLS.map((d) => d.name),
  ...ADMIN_TOOLS.map((d) => d.name),
].sort();

test("package exposes bin entry and files whitelist", () => {
  const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8")) as {
    bin?: Record<string, string>;
    files?: string[];
  };
  assert.equal(pkg.bin?.["tenara-mcp"], "./src/stdio.ts");
  assert.deepEqual(pkg.files, ["src"]);
});

test("fails fast without required env", async () => {
  const child = spawn(process.execPath, ["src/stdio.ts"], {
    cwd: new URL("..", import.meta.url).pathname,
  });
  let stderr = "";
  child.stderr?.on("data", (chunk) => {
    stderr += String(chunk);
  });
  const code = await new Promise<number>((resolve) => {
    child.on("exit", (c) => resolve(c ?? -1));
  });
  assert.notEqual(code, 0);
  assert.match(stderr, /TENARA_API_URL/);
});

test("tools/list matches remote catalog exactly", async (t) => {
  const child = spawn(process.execPath, ["src/stdio.ts"], {
    cwd: new URL("..", import.meta.url).pathname,
    env: { ...process.env, TENARA_API_URL: "http://127.0.0.1:9", TENARA_API_TOKEN: "t" },
    stdio: ["pipe", "pipe", "pipe"],
  });
  t.after(() => {
    try {
      child.kill();
    } catch {
      // already gone
    }
  });

  const chunks: string[] = [];
  child.stdout?.on("data", (chunk) => {
    chunks.push(String(chunk));
  });

  // Write both frames up front and close the write side; the transport
  // processes queued messages before honouring the EOF.
  child.stdin?.write(
    JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2025-06-18",
        capabilities: {},
        clientInfo: { name: "t", version: "0" },
      },
    }) + "\n",
  );
  child.stdin?.write(JSON.stringify({ jsonrpc: "2.0", id: 2, method: "tools/list" }) + "\n");
  child.stdin?.end();

  const names = await new Promise<string[]>((resolve, reject) => {
    const timer = setTimeout(() => {
      try {
        child.kill();
      } catch {
        // gone
      }
      reject(new Error("timeout waiting tools/list"));
    }, 7000);
    child.on("exit", (code) => {
      clearTimeout(timer);
      if (chunks.join("").includes('"id":2')) {
        try {
          for (const line of chunks.join("").split("\n")) {
            if (!line.startsWith("{")) continue;
            const msg = JSON.parse(line) as {
              id?: number;
              result?: { tools?: Array<{ name: string }> };
            };
            if (msg.id === 2 && msg.result?.tools) {
              resolve(msg.result.tools.map((tool) => tool.name).sort());
              return;
            }
          }
        } catch (err) {
          reject(err as Error);
          return;
        }
      }
      reject(new Error(`child exited ${code} before tools/list answered`));
    });
    const iv = setInterval(() => {
      const buf = chunks.join("");
      for (const line of buf.split("\n")) {
        if (!line.startsWith("{")) continue;
        try {
          const msg = JSON.parse(line) as {
            id?: number;
            result?: { tools?: Array<{ name: string }> };
          };
          if (msg.id === 2 && msg.result?.tools) {
            clearInterval(iv);
            clearTimeout(timer);
            try {
              child.kill();
            } catch {
              // gone
            }
            resolve(msg.result.tools.map((tool) => tool.name).sort());
            return;
          }
        } catch {
          // partial line; keep scanning
        }
      }
    }, 40);
  });

  assert.deepEqual(names, EXPECTED_NAMES);
});
