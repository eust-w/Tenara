import assert from "node:assert/strict";
import test from "node:test";

import { isPublicPath, needsRedirect } from "../src/lib/admin-guard.ts";

test("login is the only public path", () => {
  assert.equal(isPublicPath("/login"), true);
  assert.equal(isPublicPath("/login/x"), true);
  assert.equal(isPublicPath("/admin"), false);
});

test("unauthenticated admin routes redirect", () => {
  assert.equal(needsRedirect("/admin", false), true);
  assert.equal(needsRedirect("/login", false), false);
  assert.equal(needsRedirect("/admin", true), false);
});
