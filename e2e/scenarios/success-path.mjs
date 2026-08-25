// E2E happy path per RB§51: simulate a Codex agent deploying an app through
// the platform REST API (same endpoints as MCP tool calls). Self-contained.
// Prereq: control-plane running; kind cluster with gateway; fixture repo pushed.

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";
const TOKEN = process.env.TENARA_API_TOKEN;
if (!TOKEN) {
  console.error("env TENARA_API_TOKEN is required");
  process.exit(1);
}

async function api(method, path, body) {
  const init = {
    method,
    headers: {
      authorization: `Bearer ${TOKEN}`,
      "content-type": "application/json",
    },
  };
  if (body !== undefined) init.body = JSON.stringify(body);
  const res = await fetch(`${API}${path}`, init);
  const data = await res.json().catch(() => ({}));
  return { status: res.status, data };
}

const steps = [];
async function step(name, fn) {
  const t0 = Date.now();
  try {
    await fn();
    steps.push({ name, ms: Date.now() - t0, ok: true });
    console.log(`  ok   ${name} (${Date.now() - t0}ms)`);
  } catch (e) {
    steps.push({ name, ms: Date.now() - t0, ok: false });
    console.error(`  FAIL ${name}: ${e.message}`);
    throw e;
  }
}

const APP_NAME = `e2e-${Date.now().toString(36)}`;
const REPO_URL = process.env.TENARA_E2E_REPO ??
	new URL("../fixtures/repos/single-nextjs", import.meta.url).pathname;

// Step 1: create app
await step("app.create", async () => {
  const r = await api("POST", "/v1/apps", { name: APP_NAME, env: "prod" });
  if (r.status >= 400) throw new Error(JSON.stringify(r.data));
});

// Step 2: analyze repository
await step("app.analyze", async () => {
  const r = await api("POST", "/v1/analyze", { repo_path: REPO_URL });
  if (r.status >= 400) throw new Error(JSON.stringify(r.data));
});

// Step 3: generate plan
let planId;
await step("app.plan", async () => {
  const r = await api("GET", `/v1/apps/${APP_NAME}/plan`);
  if (r.status >= 400) throw new Error(JSON.stringify(r.data));
  planId = r.data?.id ?? r.data?.plan_id ?? r.data?.PlanID;
  if (!planId) throw new Error("no plan_id in response");
});

// Step 4: deploy approved plan (R9 implicit approval)
await step("app.deploy", async () => {
  const r = await api("POST", `/v1/apps/${APP_NAME}/deployments`, { plan_id: planId });
  if (r.status >= 400) throw new Error(JSON.stringify(r.data));
});

// Step 5: poll status until RUNNING
let finalState;
await step("poll status → RUNNING", async () => {
  for (let i = 0; i < 60; i++) {
    const r = await api("GET", `/v1/apps/${APP_NAME}`);
    finalState = r.data?.state ?? r.data?.status?.phase ?? "unknown";
    if (finalState === "RUNNING") return;
    if (finalState === "FAILED") throw new Error(`deployment FAILED: ${JSON.stringify(r.data)}`);
    await new Promise((resolve) => setTimeout(resolve, 5000));
  }
  throw new Error("timeout waiting for RUNNING");
});

// Step 6: verify HTTP reachability
const deployUrl = `https://${APP_NAME}.127.0.0.1.nip.io/`;
await step("HTTP GET deployed app", async () => {
  const res = await fetch(deployUrl, { signal: AbortSignal.timeout(15000) });
  if (res.status >= 500) throw new Error(`HTTP ${res.status}`);
  console.log(`  HTTP ${res.status} from ${deployUrl}`);
});

console.log("\n=== E2E happy path timeline ===");
for (const s of steps) console.log(`  ${s.ok ? "✓" : "✗"} ${s.name} (${s.ms}ms)`);
console.log(`\nDeployed URL: ${deployUrl}`);
console.log(`Final state: ${finalState}`);
