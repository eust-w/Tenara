import assert from "node:assert/strict";
import * as http from "node:http";
import type { AddressInfo } from "node:net";
import test from "node:test";

import { createGateway, hostAllowed, originAllowed, type GatewayDeps } from "../src/http.ts";

interface RecordedFetch {
  url: string;
  headers?: Record<string, string>;
}

function depsWith(fetchLog: RecordedFetch[] | undefined, opts?: Partial<GatewayDeps>): GatewayDeps {
  return {
    platformURL: "http://platform.test",
    allowedHosts: ["127.0.0.1", "localhost", "api.tenara.test"],
    allowedOrigins: ["https://app.tenara.test"],
    fetchImpl: async (url, init) => {
      if (fetchLog !== undefined) {
        fetchLog.push({ url, headers: init?.headers });
      }
      const auth = init?.headers?.Authorization ?? "";
      return { status: auth === "Bearer good-token" ? 200 : 401 };
    },
    ...opts,
  };
}

async function listen(t: test.TestContext, deps: GatewayDeps): Promise<string> {
  const handler = createGateway(deps);
  const srv = http.createServer((req, res) => {
    void handler(req, res);
  });
  t.after(() => {
    srv.close();
  });
  await new Promise<void>((resolve) => {
    srv.listen(0, "127.0.0.1", () => {
      resolve();
    });
  });
  const addr = srv.address() as AddressInfo;
  return `http://127.0.0.1:${addr.port}`;
}

async function post(base: string, body: string, headers: Record<string, string>) {
  const res = await fetch(`${base}/mcp`, {
    method: "POST",
    headers,
    body,
    signal: AbortSignal.timeout(4000),
  });
  const text = await res.text();
  return { status: res.status, text };
}

const rpcHeaders: Record<string, string> = {
  authorization: "Bearer good-token",
  accept: "application/json, text/event-stream",
  "content-type": "application/json",
};

// Streamable HTTP may answer plain JSON or an SSE stream; pull the JSON-RPC
// payload for the given id out of either shape.
function extractRpc(text: string, id: number): Record<string, unknown> {
  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    for (const line of text.split("\n")) {
      if (line.startsWith("data:")) {
        const candidate = JSON.parse(line.slice(5).trim()) as { id?: number };
        if (candidate.id === id) {
          return candidate;
        }
      }
    }
  }
  throw new Error(`no rpc payload for id ${id} in: ${text.slice(0, 200)}`);
}

test("healthz answers 200 ok", async (t) => {
  const base = await listen(t, depsWith(undefined));
  const res = await fetch(`${base}/healthz`, { signal: AbortSignal.timeout(4000) });
  assert.equal(res.status, 200);
  assert.equal(await res.text(), "ok");
});

test("missing bearer token yields 401", async (t) => {
  const base = await listen(t, depsWith(undefined));
  const res = await post(
    base,
    JSON.stringify({ jsonrpc: "2.0", id: 1, method: "initialize", params: {} }),
    { "content-type": "application/json", accept: "application/json, text/event-stream" },
  );
  assert.equal(res.status, 401);
});

test("forged origin yields 403 even with valid token", async (t) => {
  const log: RecordedFetch[] = [];
  const base = await listen(t, depsWith(log));
  const res = await post(
    base,
    JSON.stringify({ jsonrpc: "2.0", id: 1, method: "initialize", params: {} }),
    {
      authorization: "Bearer good-token",
      origin: "https://evil.example",
      accept: "application/json, text/event-stream",
    },
  );
  assert.equal(res.status, 403);
  assert.equal(log.length, 0, "origin guard must fire before any platform call");
});

test("initialize then tools/list expose registered tools", async (t) => {
  const base = await listen(t, depsWith(undefined));

  const initRes = await post(
    base,
    JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2025-06-18",
        capabilities: {},
        clientInfo: { name: "t", version: "0" },
      },
    }),
    rpcHeaders,
  );
  assert.equal(initRes.status, 200);
  const initRpc = extractRpc(initRes.text, 1);
  const info = (initRpc.result as { serverInfo?: { name?: string } }).serverInfo;
  assert.equal(info?.name, "tenara-mcp");

  const listRes = await post(
    base,
    JSON.stringify({ jsonrpc: "2.0", id: 2, method: "tools/list" }),
    rpcHeaders,
  );
  assert.equal(listRes.status, 200);
  const listRpc = extractRpc(listRes.text, 2);
  const tools = (listRpc.result as { tools?: Array<{ name: string }> }).tools ?? [];
  assert.ok(
    tools.some((tool) => tool.name === "ping"),
    JSON.stringify(tools),
  );
});

test("pure guards: host and origin tables", () => {
  assert.equal(hostAllowed(undefined, ["h"]), false);
  assert.equal(hostAllowed("api.tenara.test", ["api.tenara.test"]), true);
  assert.equal(hostAllowed("127.0.0.1:3000", ["127.0.0.1"]), true);
  assert.equal(hostAllowed("evil.test", ["api.tenara.test"]), false);
  assert.equal(hostAllowed("evil.test:3000", ["api.tenara.test"]), false);

  assert.equal(originAllowed(undefined, [], []), true);
  assert.equal(originAllowed("https://app.tenara.test", ["https://app.tenara.test"], []), true);
  assert.equal(originAllowed("https://app.tenara.test", [], ["app.tenara.test"]), true);
  assert.equal(originAllowed("https://evil.example", [], ["app.tenara.test"]), false);
  assert.equal(originAllowed("not-a-url", [], []), false);
});
