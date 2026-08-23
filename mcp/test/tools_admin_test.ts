import assert from "node:assert/strict";
import test from "node:test";

import { TenaraClient, type FetchLike } from "@tenara/sdk";

import {
  ADMIN_TOOLS,
  runAdminChecked,
  type AdminGate,
  type AdminToolDef,
} from "../src/tools_admin.ts";

interface RecordedCall {
  method: string;
  url: string;
  body?: string;
}

function fakeFetch(
  handler: (call: RecordedCall) => { status: number; body?: string },
): FetchLike & { calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const impl = ((
    url: string,
    init?: {
      method?: string;
      body?: string;
    },
  ) => {
    const call: RecordedCall = { url, method: init?.method ?? "GET", body: init?.body };
    calls.push(call);
    const res = handler(call);
    return Promise.resolve({ status: res.status, text: async () => res.body ?? "" });
  }) as FetchLike & { calls: RecordedCall[] };
  impl.calls = calls;
  return impl;
}

function newClient(f: FetchLike & { calls: RecordedCall[] }): TenaraClient {
  return new TenaraClient({ baseURL: "http://api.test", token: "admin-tok", fetchImpl: f });
}

const denyAll: AdminGate = { capabilityOf: async () => false };
const allowAll: AdminGate = { capabilityOf: async () => true };

function adminDef(name: string): AdminToolDef {
  const d = ADMIN_TOOLS.find((x) => x.name === name);
  assert.ok(d, `${name} missing`);
  return d;
}

test("catalog has exactly the seven admin tools", () => {
  assert.deepEqual(
    ADMIN_TOOLS.map((d) => d.name),
    [
      "admin.user.list",
      "admin.user.suspend",
      "admin.app.list",
      "admin.app.stop",
      "admin.quota.set",
      "admin.cluster.health",
      "admin.security.events",
    ],
  );
});

test("no raw cluster objects surface in metadata", () => {
  const blob = JSON.stringify(
    ADMIN_TOOLS.map((d) => ({
      name: d.name,
      description: d.description,
      shape: d.shape,
    })),
  );
  for (const banned of ["kubectl", "etcd", "raw-object"]) {
    assert.equal(blob.includes(banned), false, `banned token ${banned}`);
  }
});

test("denied member gets forbidden with zero network calls", async () => {
  for (const def of ADMIN_TOOLS) {
    const args: Record<string, string> =
      def.name === "admin.user.suspend"
        ? { user_id: "u9" }
        : def.name.includes("app.") || def.name === "admin.quota.set"
          ? { app_id: "a1" }
          : {};
    const f = fakeFetch(() => ({ status: 200 }));
    const out = await runAdminChecked(def, args, newClient(f), denyAll, "member-tok");

    assert.ok(out.human.includes("forbidden"), `${def.name} human = ${out.human}`);
    assert.equal(out.failed, true, `${def.name} must mark failure`);
    const payload = out.data as { error?: string; capability?: string };
    assert.equal(payload.error, "forbidden");
    assert.ok((payload.capability ?? "").startsWith("admin."));
    assert.equal(f.calls.length, 0, `${def.name} must issue zero requests when denied`);
  }
});

test("allowed admin walks expected verbs and paths", async () => {
  const cases: Array<{
    name: string;
    method: string;
    suffix: string;
    args?: Record<string, string>;
  }> = [
    { name: "admin.user.list", method: "GET", suffix: "/v1/admin/users" },
    {
      name: "admin.user.suspend",
      method: "POST",
      suffix: "/v1/admin/users/u9/suspend",
      args: { user_id: "u9" },
    },
    { name: "admin.app.list", method: "GET", suffix: "/v1/admin/apps" },
    {
      name: "admin.app.stop",
      method: "POST",
      suffix: "/v1/admin/apps/a1/stop",
      args: { app_id: "a1" },
    },
    {
      name: "admin.quota.set",
      method: "PUT",
      suffix: "/v1/admin/apps/a1/quota",
      args: { app_id: "a1" },
    },
    { name: "admin.cluster.health", method: "GET", suffix: "/v1/admin/cluster/health" },
    { name: "admin.security.events", method: "GET", suffix: "/v1/admin/security-events" },
  ];
  for (const tc of cases) {
    const f = fakeFetch(() => ({ status: 200, body: "{}" }));
    const out = await runAdminChecked(
      adminDef(tc.name),
      tc.args ?? {},
      newClient(f),
      allowAll,
      "tok",
    );
    assert.equal(out.failed, undefined, `${tc.name} allowed run must not fail`);
    const call = f.calls.at(-1);
    assert.ok(call, `${tc.name} produced no call`);
    assert.equal(call.method, tc.method, `${tc.name} verb`);
    assert.ok(call.url.endsWith(tc.suffix), `${tc.name} url ${call.url}`);
  }
});

test("quota set carries tier in body", async () => {
  const f = fakeFetch(() => ({ status: 200, body: "{}" }));
  await runAdminChecked(
    adminDef("admin.quota.set"),
    { app_id: "a1", tier: "pro" },
    newClient(f),
    allowAll,
    "tok",
  );
  const call = f.calls.at(-1);
  assert.ok(call?.body !== undefined, "quota set must carry a body");
  assert.equal((JSON.parse(call.body) as { tier?: string }).tier, "pro");
});
