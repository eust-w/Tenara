import assert from "node:assert/strict";
import test from "node:test";

import { isAppScoped, listPath } from "../src/lib/settings-endpoints.ts";
import { maskSecretPayload, maskSecretValues } from "../src/lib/settings-mask.ts";

test("list paths map per resource and scope", () => {
  assert.equal(listPath("domains", "app-1"), "/v1/apps/app-1/domains");
  assert.equal(listPath("databases", "app-1"), "/v1/apps/app-1/databases");
  assert.equal(listPath("secrets", "app-1"), "/v1/apps/app-1/env");
  assert.equal(listPath("tokens", null), "/v1/org/tokens");
  assert.equal(listPath("members", null), "/v1/org/members");
});

test("scope classification", () => {
  for (const r of ["domains", "databases", "secrets"] as const) {
    assert.equal(isAppScoped(r), true);
  }
  for (const r of ["tokens", "members"] as const) {
    assert.equal(isAppScoped(r), false);
  }
});

test("secret masking never echoes original values", () => {
  const masked = maskSecretValues({ MONGO_URI: "mongodb://root:pw@h/m", API_KEY: "sk-live" });
  assert.deepEqual(masked, { MONGO_URI: "configured", API_KEY: "configured" });
});

test("payload masking keeps non-secret fields untouched", () => {
  const out = maskSecretPayload({
    values: { K: "plain" },
    name: "prod",
  }) as { values: Record<string, string>; name: string };
  assert.equal(out.values.K, "configured");
  assert.equal(out.name, "prod");
});
