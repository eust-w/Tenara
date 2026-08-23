import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { TenaraClient, type FetchLike } from "../src/client.ts";

interface RecordedCall {
  url: string;
  method: string;
  headers: Record<string, string>;
}

type Handler = (callNo: number, call: RecordedCall) => { status: number; body?: string };

function firstCall(f: { calls: RecordedCall[] }): RecordedCall {
  const c = f.calls[0];
  assert.ok(c, "expected at least one recorded call");
  return c;
}

function fakeFetch(handler: Handler): FetchLike & { calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const impl = ((url: string, init?: { method?: string; headers?: Record<string, string> }) => {
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

function newClient(f: ReturnType<typeof fakeFetch>): TenaraClient {
  return new TenaraClient({
    baseURL: "http://api.test",
    token: "t0k",
    fetchImpl: f,
    idempotencyKeyFactory: () => "key-123",
    maxRetries: 3,
    backoffMs: 0,
  });
}

test("POST carries bearer token and auto-generated idempotency key", async () => {
  const f = fakeFetch(() => ({ status: 201, body: '{"id":"app-1"}' }));
  const client = newClient(f);

  const res = await client.post<{ id: string }>("/v1/apps", { name: "x" });

  assert.equal(res.status, 201);
  assert.deepEqual(res.data, { id: "app-1" });
  const call = firstCall(f);
  assert.equal(call.url, "http://api.test/v1/apps");
  assert.equal(call.method, "POST");
  assert.equal(call.headers.Authorization, "Bearer t0k");
  assert.equal(call.headers["Idempotency-Key"], "key-123");
});

test("explicit idempotency key passes through over the factory", async () => {
  const f = fakeFetch(() => ({ status: 202, body: "" }));
  const client = newClient(f);

  await client.post("/v1/apps/app-1/deploy", { plan_id: "p1" }, "my-explicit-key");

  assert.equal(firstCall(f).headers["Idempotency-Key"], "my-explicit-key");
});

test("GET skips idempotency header but keeps authorization", async () => {
  const f = fakeFetch(() => ({ status: 200, body: "[]" }));
  const client = newClient(f);

  await client.get("/v1/apps");

  assert.equal(firstCall(f).method, "GET");
  assert.equal(firstCall(f).headers.Authorization, "Bearer t0k");
  assert.equal("Idempotency-Key" in firstCall(f).headers, false);
});

test("retries 429 up to maxRetries on idempotent methods", async () => {
  let n = 0;
  const f = fakeFetch(() => {
    n += 1;
    return n < 3 ? { status: 429 } : { status: 200, body: '"ok"' };
  });
  const client = newClient(f);

  const res = await client.get("/v1/apps");

  assert.equal(res.status, 200);
  assert.equal(f.calls.length, 3);
});

test("never retries non-idempotent methods even on 429", async () => {
  const f = fakeFetch(() => ({ status: 429 }));
  const client = newClient(f);

  const res = await client.post("/v1/apps", { name: "x" });

  assert.equal(res.status, 429);
  assert.equal(f.calls.length, 1);
});

test("retries 5xx on DELETE until success", async () => {
  let n = 0;
  const f = fakeFetch(() => {
    n += 1;
    return n === 1 ? { status: 502 } : { status: 204 };
  });
  const client = newClient(f);

  const res = await client.del("/v1/apps/app-1");

  assert.equal(res.status, 204);
  assert.equal(f.calls.length, 2);
});

test("does not retry 4xx client errors", async () => {
  const f = fakeFetch(() => ({ status: 400, body: '{"error":"bad"}' }));
  const client = newClient(f);

  const res = await client.get("/v1/nope");

  assert.equal(res.status, 400);
  assert.equal(f.calls.length, 1);
});

test("client source never references localStorage", () => {
  const src = readFileSync(new URL("../src/client.ts", import.meta.url), "utf8");
  assert.equal(src.includes("localStorage"), false);
});

test("generated openapi types are present", () => {
  const gen = readFileSync(new URL("../src/types.gen.ts", import.meta.url), "utf8");
  assert.ok(gen.includes("/v1/apps"));
  assert.ok(gen.length > 1000);
});
