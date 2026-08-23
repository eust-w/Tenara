import assert from "node:assert/strict";
import test from "node:test";

import { passwordIssue } from "../src/lib/password-policy.ts";

test("password policy mirrors backend rules", () => {
  assert.match(String(passwordIssue("short1")), /8 characters/);
  assert.match(String(passwordIssue("nonumbers")), /letters and numbers/);
  assert.match(String(passwordIssue("12345678")), /letters and numbers/);
  assert.equal(passwordIssue("goodpass1"), null);
});
