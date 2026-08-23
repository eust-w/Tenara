import assert from "node:assert/strict";
import test from "node:test";

import { canDeploy, isUnsupportedStack, supportedRuntimes } from "../src/lib/plan-view.ts";

test("unsupported stack detection and runtime list extraction", () => {
  assert.equal(isUnsupportedStack({ code: "UNSUPPORTED_STACK" }), true);
  assert.equal(isUnsupportedStack({ unsupported: { code: "UNSUPPORTED_STACK" } }), true);
  assert.equal(isUnsupportedStack({ state: "RUNNING" }), false);
  assert.deepEqual(supportedRuntimes({ unsupported: { supported: ["next", "fastapi", "go"] } }), [
    "next",
    "fastapi",
    "go",
  ]);
});

test("deploy button only approves a PLANNED plan (R9)", () => {
  assert.equal(canDeploy(null), false);
  assert.equal(canDeploy(undefined), false);
  assert.equal(canDeploy({}), false);
  assert.equal(canDeploy({ id: "p1", state: "AWAITING_APPROVAL" }), false);
  assert.equal(canDeploy({ id: "p1", state: "PLANNED" }), true);
});
