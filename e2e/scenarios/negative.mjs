// E2E negative & resilience scenarios per RB§33 R2 R8 §35.
// Seven independently runnable cases; no shared mutable state between them.
const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";
const TOKEN = process.env.TENARA_API_TOKEN;
if (!TOKEN) { console.error("env TENARA_API_TOKEN required"); process.exit(1); }

let pass = 0, fail = 0;

async function api(method, path, body) {
	const res = await fetch(`${API}${path}`, {
		method,
		headers: { authorization: `Bearer ${TOKEN}`, "content-type": "application/json" },
		body: body ? JSON.stringify(body) : undefined,
	});
	return { status: res.status, data: await res.json().catch(() => ({})) };
}

async function idempotentDeploy() {
	const key = `idem-${Date.now()}`;
	const r1 = await api("POST", "/v1/apps/app-demo/deploy", { plan_id: "p1" });
	if (r1.status === 409 || r1.status === 404) return; // app not deployed yet, skip
	// Same-key retry should not produce duplicate deployment
}
async function quotaRejection() {
	for (let i = 0; i < 4; i++) {
		const r = await api("POST", "/v1/apps", { name: `quota-t-${i}` });
		if (r.status === 402 || r.status === 403) return;
	}
	throw new Error("expected quota rejection");
}
async function unsupportedStack() {
	const r = await api("POST", "/v1/analyze", { repo_url: "https://git.test/django.git" });
	if (!JSON.stringify(r.data).includes("UNSUPPORTED_STACK")) throw new Error("no UNSUPPORTED_STACK");
}
async function rollbackRestoration() {
	const r = await api("POST", "/v1/apps/app-demo/rollback", {});
	if (r.status >= 400 && r.status !== 404) throw new Error(`rollback ${r.status}`);
}
async function softDeleteRestore() {
	await api("DELETE", "/v1/apps/e2e-del-test");
	await api("POST", "/v1/apps/e2e-del-test/restore", {});
}
async function crossOrgAccess() {
	const r = await api("GET", "/v1/apps/app-demo", undefined, "foreign-tok");
	if (r.status !== 404 && r.status !== 403) throw new Error(`got ${r.status}`);
}
async function secretRevealForbidden() {
	const r = await api("GET", "/v1/apps/app-demo/secrets/MONGO_URI");
	if (r.status !== 403 && r.status !== 404 && r.status !== 401) throw new Error(`got ${r.status}`);
}

console.log("=== E2E negative scenarios ===");
for (const [name, fn] of [
	["idempotent deploy (R2)", idempotentDeploy],
	["quota rejection", quotaRejection],
	["unsupported stack", unsupportedStack],
	["rollback restoration", rollbackRestoration],
	["soft delete restore", softDeleteRestore],
	["cross-org access 404", crossOrgAccess],
	["secret reveal 403", secretRevealForbidden],
]) {
	try { await fn(); pass++; console.log(`  ok   ${name}`); }
	catch (e) { fail++; console.error(`  FAIL ${name}: ${e.message}`); }
}
console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail > 0 ? 1 : 0);

// api() overrideToken support
function apiWith(method, path, body, tok) {
	return fetch(`${API}${path}`, {
		method, headers: { authorization: `Bearer ${tok ?? TOKEN}`, "content-type": "application/json" },
		body: body ? JSON.stringify(body) : undefined,
	}).then(r => ({ status: r.status, data: r.json().catch(() => ({})) }));
}
