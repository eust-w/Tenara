import assert from "node:assert/strict";
import test from "node:test";

import { filterRepos, normalizeRepoList } from "../src/lib/repos.ts";

test("normalizes github payload and drops unknown fields", () => {
  const items = normalizeRepoList([
    {
      full_name: "acme/widget",
      private: true,
      default_branch: "trunk",
      token: "SHOULD-NOT-LEAK",
      permissions: { admin: true },
    },
    { full_name: "" },
  ]);
  assert.equal(items.length, 1);
  assert.deepEqual(Object.keys(items[0]).sort(), ["defaultBranch", "fullName", "privateRepo"]);
  assert.equal(items[0].defaultBranch, "trunk");
});

test("filters repos by case-insensitive substring", () => {
  const repos = normalizeRepoList([
    { full_name: "acme/widget" },
    { full_name: "acme/Gadget" },
    { full_name: "other/thing" },
  ]);
  assert.deepEqual(
    filterRepos(repos, "ga").map((r) => r.fullName),
    ["acme/Gadget"],
  );
  assert.equal(filterRepos(repos, "  ").length, 3);
});

test("empty or non-array payloads yield empty list", () => {
  assert.deepEqual(normalizeRepoList(null), []);
  assert.deepEqual(normalizeRepoList("nope"), []);
});
