// MCP protocol conformance (RB§8 D4): the remote streamable-HTTP gateway and
// the local stdio wrapper must answer identical protocol cases identically.
// Self-contained script — exits non-zero on any failure.
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import * as http from "node:http";
import { fileURLToPath } from "node:url";

import { createGateway } from "../../mcp/src/http.ts";
import { ADMIN_TOOLS } from "../../mcp/src/tools_admin.ts";
import { APP_TOOLS } from "../../mcp/src/tools.ts";

const mcpDir = fileURLToPath(new URL("../../mcp", import.meta.url));
const docPath = fileURLToPath(new URL("../../docs/mcp-onboarding.md", import.meta.url));

const EXPECTED_NAMES = [
  "ping",
  ...APP_TOOLS.map((d) => d.name),
  ...ADMIN_TOOLS.map((d) => d.name),
].sort();

let failures = 0;
function check(label, cond, detail = "") {
  if (cond) {
    console.log(`  ok   ${label}`);
  } else {
    failures += 1;
    console.error(`  FAIL ${label} ${detail}`);
  }
}

async function listen() {
  const handler = createGateway({
    platformURL: "http://platform.test",
    allowedHosts: ["127.0.0.1", "localhost"],
    allowedOrigins: [],
    fetchImpl: async (url, init) => {
      const auth = (init?.headers ?? {}).Authorization ?? "";
      return { status: auth === "Bearer good-token" ? 200 : 401 };
    },
  });
  const srv = http.createServer((req, res) => {
    void handler(req, res);
  });
  await new Promise((resolve) => srv.listen(0, "127.0.0.1", resolve));
  const { port } = srv.address();
  testCleanup.push(() => srv.close());
  return `http://127.0.0.1:${port}`;
}

const testCleanup = [];

function extractRpc(text, id) {
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    const candidate = trimmed.startsWith("data:") ? trimmed.slice(5).trim() : trimmed;
    if (!candidate.startsWith("{")) continue;
    try {
      const msg = JSON.parse(candidate);
      if (msg.id === id) return msg;
    } catch {
      // partial or unrelated line
    }
  }
  return null;
}

async function rpcRemote(base, id, method, params = {}) {
  const res = await fetch(`${base}/mcp`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      accept: "application/json, text/event-stream",
      authorization: "Bearer good-token",
    },
    body: JSON.stringify({ jsonrpc: "2.0", id, method, ...(params ? { params } : {}) }),
    signal: AbortSignal.timeout(6000),
  });
  const text = await res.text();
  const msg = extractRpc(text, id);
  check(
    `remote rpc id=${id} answered`,
    msg !== null,
    `status=${res.status} body=${text.slice(0, 160)}`,
  );
  return { status: res.status, msg: msg ?? {} };
}

async function collectStdio() {
  const child = spawn(process.execPath, ["src/stdio.ts"], {
    cwd: mcpDir,
    env: { ...process.env, TENARA_API_URL: "http://127.0.0.1:9", TENARA_API_TOKEN: "t" },
    stdio: ["pipe", "pipe", "pipe"],
  });
  let out = "";
  child.stdout.on("data", (chunk) => {
    out += String(chunk);
  });
  const done = new Promise((resolve) => child.on("exit", resolve));
  return { child, done, getText: () => out };
}

function frame(id, method, params) {
  return JSON.stringify({ jsonrpc: "2.0", id, method, ...(params ? { params } : {}) }) + "\n";
}

console.log("[1] remote protocol cases");
{
  const base = await listen();

  const init = await rpcRemote(base, 1, "initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "t", version: "0" },
  });
  check("remote initialize reports tenara-mcp", init.msg.result?.serverInfo?.name === "tenara-mcp");

  const list = await rpcRemote(base, 2, "tools/list");
  const names = (list.msg.result?.tools ?? []).map((tool) => tool.name).sort();
  check(
    "remote tools/list matches catalog",
    JSON.stringify(names) === JSON.stringify(EXPECTED_NAMES),
  );

  const ping = await rpcRemote(base, 3, "tools/call", { name: "ping", arguments: {} });
  check(
    "remote ping returns pong",
    ping.msg.result?.content?.[0]?.text === "pong" && ping.msg.result?.isError !== true,
  );

  const bogus = await rpcRemote(base, 4, "tools/call", { name: "does-not-exist", arguments: {} });
  check(
    "remote unknown tool surfaces an error",
    (bogus.msg.error?.code ?? 0) < 0 || bogus.msg.result?.isError === true,
  );
}

console.log("[2] stdio protocol cases");
{
  const { child, done, getText } = await collectStdio();
  child.stdin.write(
    frame(1, "initialize", {
      protocolVersion: "2025-06-18",
      capabilities: {},
      clientInfo: { name: "t", version: "0" },
    }) +
      frame(2, "tools/list") +
      frame(3, "tools/call", { name: "ping", arguments: {} }) +
      frame(4, "tools/call", { name: "does-not-exist", arguments: {} }),
  );
  child.stdin.end();

  const killer = setTimeout(() => child.kill(), 9000);
  await Promise.race([done, new Promise((r) => setTimeout(r, 9000))]);
  clearTimeout(killer);

  const out = getText();
  const msgs = [];
  for (const line of out.split("\n")) {
    if (!line.startsWith("{")) continue;
    try {
      msgs.push(JSON.parse(line));
    } catch {
      // partial
    }
  }
  const byId = (id) => msgs.find((m) => m.id === id) ?? {};

  check("stdio initialize reports tenara-mcp", byId(1).result?.serverInfo?.name === "tenara-mcp");
  check(
    "stdio tools/list matches catalog",
    JSON.stringify((byId(2).result?.tools ?? []).map((tool) => tool.name).sort()) ===
      JSON.stringify(EXPECTED_NAMES),
  );
  check(
    "stdio ping returns pong",
    byId(3).result?.content?.[0]?.text === "pong" && byId(3).result?.isError !== true,
  );
  check(
    "stdio unknown tool surfaces an error",
    (byId(4).error?.code ?? 0) < 0 || byId(4).result?.isError === true,
  );
  try {
    child.kill();
  } catch {
    // gone
  }
}

console.log("[3] parity between transports");
{
  const base = await listen();
  const remoteList = await rpcRemote(base, 2, "tools/list");
  const remoteNames = (remoteList.msg.result?.tools ?? []).map((tool) => tool.name).sort();

  const { child, done, getText } = await collectStdio();
  child.stdin.write(frame(2, "tools/list"));
  child.stdin.end();
  await Promise.race([done, new Promise((r) => setTimeout(r, 6000))]);
  child.kill();

  const stdioNames = getText()
    .split("\n")
    .filter((line) => line.startsWith("{"))
    .map((line) => {
      try {
        return (JSON.parse(line).result?.tools ?? []).map((tool) => tool.name);
      } catch {
        return [];
      }
    })
    .flat()
    .sort();
  check(
    "tool-set diff empty across transports",
    JSON.stringify(remoteNames) === JSON.stringify(stdioNames),
  );
}

console.log("[4] onboarding doc safety and shape");
{
  const md = readFileSync(docPath, "utf8");
  check("code fences balanced", (md.match(/```/g) ?? []).length % 2 === 0);
  check("codex sample pins type=http", /type(\s*)=(\s*)"http"/.test(md));
  check("claude sample pins type=http", /"type"\s*:\s*"http"/.test(md));
  for (const banned of ["ghp_", "sk-", "sup3rsecret"]) {
    check(`doc free of ${banned}`, !md.includes(banned));
  }
}

for (const fn of testCleanup.splice(0)) fn();

if (failures > 0) {
  console.error(`conformance FAILED: ${failures} check(s)`);
  process.exit(1);
}
console.log("conformance PASSED");
process.exit(0);
