import assert from "node:assert/strict";
import test from "node:test";

import { isPublicPath, needsRedirect } from "../src/lib/auth-guard.ts";

test("public paths are open without a session", () => {
  for (const p of ["/login", "/register", "/verify", "/forgot", "/login/x"]) {
    assert.equal(isPublicPath(p), true, p);
  }
});

test("protected paths redirect when no session cookie", () => {
  assert.equal(needsRedirect("/", false), true);
  assert.equal(needsRedirect("/apps/app-1", false), true);
  assert.equal(needsRedirect("/", true), false);
  assert.equal(needsRedirect("/login", false), false);
});
