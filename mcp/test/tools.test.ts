import assert from "node:assert/strict";
import test from "node:test";

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { TenaraClient, type FetchLike } from "@tenara/sdk";

import { APP_TOOLS, dualContent, registerAppTools, type ToolDef } from "../src/tools.ts";

interface RecordedCall {
  method: string;
  url: string;
  headers: Record<string, string>;
}

type Handler = (callNo: number, call: RecordedCall) => { status: number; body?: string };

function fakeFetch(handler: Handler): FetchLike & { calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const impl = ((
    url: string,
    init?: {
      method?: string;
      headers?: Record<string, string>;
    },
  ) => {
    const call: RecordedCall = {
      url,
      method: init?.method ?? "GET",
      headers: init?.headers ?? {},
    };
    calls.push(call);
    const res = handler(calls.length, call);
    return Promise.resolve({ status: res.status, text: async () => res.body ?? "" });
  }) as FetchLike & { calls: RecordedCall[] };
  impl.calls = calls;
  return impl;
}

function newClient(f: FetchLike & { calls: RecordedCall[] }): TenaraClient {
  return new TenaraClient({
    baseURL: "http://api.test",
    token: "t0k",
    fetchImpl: f,
    idempotencyKeyFactory: () => `idem-${f.calls.length + 1}`,
    maxRetries: 0,
    backoffMs: 0,
  });
}

function toolDef(name: string): ToolDef {
  const d = APP_TOOLS.find((x) => x.name === name);
  assert.ok(d, `${name} missing from catalog`);
  return d;
}

const WANT_NAMES = [
  "app.create",
  "app.analyze",
  "app.plan",
  "app.deploy",
  "app.status",
  "app.logs",
  "app.restart",
  "app.rollback",
  "app.delete",
  "app.set_env",
  "database.create",
  "domain.bind",
];

const MATRIX = [
  {
    name: "app.create",
    args: { name: "demo" },
    method: "POST",
    suffix: "/v1/apps",
    status: 201,
    body: '{"id":"app-demo"}',
  },
  {
    name: "app.analyze",
    args: { repo_url: "https://git.test/x.git" },
    method: "POST",
    suffix: "/v1/analyze",
    status: 200,
    body: "{}",
  },
  {
    name: "app.plan",
    args: { app_id: "app-demo" },
    method: "POST",
    suffix: "/v1/apps/app-demo/plan",
    status: 200,
    body: '{"state":"PLANNED"}',
  },
  {
    name: "app.deploy",
    args: { app_id: "app-demo", plan_id: "p1" },
    method: "POST",
    suffix: "/v1/apps/app-demo/deploy",
    status: 202,
    body: '{"state":"DEPLOYING"}',
  },
  {
    name: "app.status",
    args: { app_id: "app-demo" },
    method: "GET",
    suffix: "/v1/apps/app-demo",
    status: 200,
    body: '{"state":"RUNNING"}',
  },
  {
    name: "app.logs",
    args: { app_id: "app-demo", tail: 50 },
    method: "GET",
    suffix: "/v1/apps/app-demo/logs?tail=50",
    status: 200,
    body: "[]",
  },
  {
    name: "app.restart",
    args: { app_id: "app-demo" },
    method: "POST",
    suffix: "/v1/apps/app-demo/restart",
    status: 202,
    body: "{}",
  },
  {
    name: "app.rollback",
    args: { app_id: "app-demo" },
    method: "POST",
    suffix: "/v1/apps/app-demo/rollback",
    status: 202,
    body: "{}",
  },
  {
    name: "app.delete",
    args: { app_id: "app-demo" },
    method: "DELETE",
    suffix: "/v1/apps/app-demo",
    status: 202,
    body: "",
  },
  {
    name: "app.set_env",
    args: { app_id: "app-demo", values: { MONGO_URI: "x" } },
    method: "PUT",
    suffix: "/v1/apps/app-demo/env",
    status: 200,
    body: "{}",
  },
  {
    name: "database.create",
    args: { app_id: "app-demo", kind: "mongo" },
    method: "POST",
    suffix: "/v1/apps/app-demo/databases",
    status: 201,
    body: "{}",
  },
  {
    name: "domain.bind",
    args: { app_id: "app-demo", hostname: "demo.test" },
    method: "POST",
    suffix: "/v1/apps/app-demo/domains",
    status: 201,
    body: "{}",
  },
];

test("catalog has exactly the twelve unique tools in order", () => {
  assert.deepEqual(
    APP_TOOLS.map((d) => d.name),
    WANT_NAMES,
  );
  for (const d of APP_TOOLS) {
    assert.ok(d.description.length > 0, `${d.name} lacks description`);
    assert.ok(Object.keys(d.shape).length > 0, `${d.name} lacks schema`);
  }
});

test("no kubernetes primitive leaks into any tool surface", () => {
  const blob = JSON.stringify(
    APP_TOOLS.map((d) => ({
      name: d.name,
      description: d.description,
      shape: d.shape,
    })),
  );
  for (const banned of [
    "kubectl",
    "configmap",
    "ingress",
    "podName",
    "nodeName",
    "volumeMount",
    "namespace",
  ]) {
    assert.equal(blob.includes(banned), false, `banned k8s token ${banned} found`);
  }
});

test("happy matrix hits expected method and path with auth headers", async () => {
  for (const tc of MATRIX) {
    const f = fakeFetch(() => ({ status: tc.status, body: tc.body }));
    const def = toolDef(tc.name);
    const out = await def.run(structuredClone(tc.args), newClient(f));

    const lastCall = f.calls.at(-1);
    assert.ok(lastCall, `${tc.name} produced no call`);
    assert.equal(lastCall.method, tc.method, tc.name);
    assert.ok(lastCall.url.endsWith(tc.suffix), `${tc.name} url ${lastCall.url}`);
    assert.equal(lastCall.headers.Authorization, "Bearer t0k", tc.name);
    if (lastCall.method === "GET") {
      assert.equal(
        "Idempotency-Key" in lastCall.headers,
        false,
        `${tc.name} GET must not carry an idempotency key`,
      );
    } else {
      assert.ok(
        (lastCall.headers["Idempotency-Key"] ?? "").length > 0,
        `${tc.name} missing idempotency key`,
      );
    }
    assert.ok(out.human.length > 0, `${tc.name} empty human block`);
    assert.notEqual(out.data, undefined, `${tc.name} empty data block`);
  }
});

test("set_env masks values as configured only", async (t) => {
  const secret = "mongodb://root:sup3r@db:27017/m";
  const f = fakeFetch(() => ({ status: 200, body: "{}" }));

  const out = await toolDef("app.set_env").run(
    { app_id: "app-demo", values: { MONGO_URI: secret } },
    newClient(f),
  );
  t.assert.ok(out.human.includes("MONGO_URI: configured"));

  const blob = JSON.stringify(out.data);
  t.assert.equal(blob.includes(secret), false);
  t.assert.deepEqual((out.data as { values: Record<string, string> }).values, {
    MONGO_URI: "configured",
  });
});

test("deploy carries rotating idempotency keys (R2)", async () => {
  let seq = 0;
  const f = fakeFetch(() => ({ status: 202 }));
  const client = new TenaraClient({
    baseURL: "http://api.test",
    token: "t0k",
    fetchImpl: f,
    idempotencyKeyFactory: () => `idem-${++seq}`,
    maxRetries: 0,
    backoffMs: 0,
  });
  const def = toolDef("app.deploy");
  const args = { app_id: "app-demo", plan_id: "p1" };

  await def.run(args, client);
  await def.run(args, client);

  const key1 = f.calls[0]?.headers["Idempotency-Key"];
  const key2 = f.calls[1]?.headers["Idempotency-Key"];
  assert.equal(key1, "idem-1");
  assert.equal(key2, "idem-2");
});

test("dualContent emits text plus pretty json blocks", () => {
  const out = dualContent("done", { a: 1 });
  assert.equal(out.content.length, 2);
  assert.equal(out.content[0]?.type, "text");
  assert.equal(out.content[0]?.text, "done");
  assert.equal((JSON.parse(out.content[1]?.text ?? "{}") as { a?: number }).a, 1);
});

test("registerAppTools wires every tool without throwing", () => {
  const server = new McpServer({ name: "smoke", version: "0.0.0" });
  registerAppTools(server, newClient(fakeFetch(() => ({ status: 200 }))));
});
