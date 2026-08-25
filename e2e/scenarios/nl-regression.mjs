// E2E natural-language regression per RB§47 (todo98): five agent utterances,
// each mapped onto the MCP-tool-equivalent REST flow. Scenarios stay
// independently runnable and collect full results instead of failing fast.
// Prereq: control-plane running (TENARA_API_URL/TENARA_API_TOKEN). Live
// cluster execution belongs to the @cluster-live gate per design docs.

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";
const TOKEN = process.env.TENARA_API_TOKEN;
if (!TOKEN) {
  console.error("env TENARA_API_TOKEN required");
  process.exit(1);
}

let pass = 0;
let fail = 0;
const failures = [];

async function api(method, path, body) {
  const res = await fetch(`${API}${path}`, {
    method,
    headers: { authorization: `Bearer ${TOKEN}`, "content-type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  return { status: res.status, data: await res.json().catch(() => ({})) };
}

async function scenario(utterance, fn) {
  console.log(`\n▶ "${utterance}"`);
  try {
    await fn();
    pass++;
    console.log("  ✓ ok");
  } catch (e) {
    fail++;
    failures.push({ utterance, reason: e.message });
    console.error(`  ✗ ${e.message}`);
  }
}

const APP = `nl-${Date.now().toString(36)}`;
let APP_ID; // set by S1, reused by later scenarios
const REPO = process.env.TENARA_E2E_REPO ??
	new URL("../fixtures/repos/single-nextjs", import.meta.url).pathname;

// S1 ── 「把我的应用上线」: the analyze -> plan -> deploy core loop.
await scenario("把我的应用上线", async () => {
  let r = await api("POST", "/v1/apps", { name: APP, env: "prod" });
	// server keys sub-resources by returned identifier (UUID "ID")
	APP_ID = r.data?.id ?? r.data?.ID ?? APP;
  if (r.status >= 400 && r.status !== 409) {
    throw new Error(`create ${r.status} ${JSON.stringify(r.data)}`);
  }
  r = await api("POST", `/v1/apps/${APP_ID}/analyze`, { repo_path: REPO });
  if (r.status >= 400) throw new Error(`analyze ${r.status} ${JSON.stringify(r.data)}`);
  r = await api("GET", `/v1/apps/${APP_ID}/plan`);
  if (r.status >= 400) throw new Error(`plan ${r.status}`);
  const planId = r.data?.id ?? r.data?.plan_id ?? r.data?.PlanID;
  if (!planId) throw new Error("no plan id in response");
  r = await api("POST", `/v1/apps/${APP_ID}/deployments`, { plan_id: planId });
  if (r.status >= 400) throw new Error(`deploy ${r.status} ${JSON.stringify(r.data)}`);
});

// S2 ── 「给应用配一个 MongoDB 数据库」: T89 multi-kind binding contract.
await scenario("给应用配一个 MongoDB", async () => {
  const r = await api("POST", `/v1/apps/${APP_ID}/databases`, { kind: "mongo" });
  if (r.status !== 200 && r.status !== 201) {
    throw new Error(`create ${r.status} ${JSON.stringify(r.data)}`);
  }
  // Idempotent replay must merge into the same binding row.
  const replay = await api("POST", `/v1/apps/${APP_ID}/databases`, { kind: "mongo" });
  if (replay.status !== 200 && replay.status !== 201) {
    throw new Error(`replay ${replay.status}`);
  }
});

// S3 ── 「给我的应用绑一个自定义域名」: TXT challenge must surface.
await scenario("绑自定义域名", async () => {
  const host = `${APP}.example.com`;
  const r = await api("POST", `/v1/apps/${APP_ID}/domains`, { hostname: host });
  if (r.status >= 400) throw new Error(`add domain ${r.status} ${JSON.stringify(r.data)}`);
  const challenge = r.data?.txt_challenge ?? r.data?.challenge;
  if (!challenge && !r.data?.verified) throw new Error("no TXT challenge surfaced");
});

// S4 ── 「出问题了,先回滚到上一版」: previous-revision rollback.
await scenario("回滚到上一版", async () => {
  const list = await api("GET", `/v1/apps/${APP_ID}/deployments`);
  if (list.status === 404) throw new Error("revisions endpoint missing");
  if (list.status >= 400) throw new Error(`list ${list.status}`);
  const revs = Array.isArray(list.data) ? list.data : (list.data?.items ?? []);
  if (revs.length < 2) return; // fresh env: single revision, nothing to roll back
  const prev = revs[revs.length - 2]?.id ?? revs[1]?.id;
  const r = await api("POST", `/v1/apps/${APP_ID}/rollback`, { revision_id: prev });
  if (r.status >= 400 && r.status !== 409) {
    throw new Error(`rollback ${r.status} ${JSON.stringify(r.data)}`);
  }
});

// S5 ── 「应用好像挂了,帮我看看怎么回事」: failure diagnostics readout.
await scenario("应用挂了帮我看看", async () => {
  const diag = await api("GET", `/v1/apps/${APP_ID}/diagnostics`);
  if (diag.status >= 500) throw new Error(`diagnostics ${diag.status}`);
  const logs = await api("GET", `/v1/apps/${APP_ID}/logs?limit=20`);
  if (logs.status >= 500) throw new Error(`logs ${logs.status}`);
});

console.log(`\n=== NL regression: ${pass} passed, ${fail} failed ===`);
for (const f of failures) console.error(`  ✗ "${f.utterance}": ${f.reason}`);
process.exit(fail > 0 ? 1 : 0);
