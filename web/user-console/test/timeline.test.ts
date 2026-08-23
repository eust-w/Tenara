import assert from "node:assert/strict";
import test from "node:test";

import { buildTimeline, logsQuery, rollbackConfirmText } from "../src/lib/timeline.ts";

test("build timeline maps fields and sorts newest first", () => {
  const rows = buildTimeline([
    { number: 1, digest: "sha256:aaaa1111bbbb", state: "PUSHED" },
    { number: 2, digest: "sha256:cccc2222dddd", state: "READY" },
  ]);
  assert.deepEqual(
    rows.map((r) => r.number),
    [2, 1],
  );
  assert.equal(rows[0].shaShort, "cccc222");
  assert.equal(rows[0].state, "READY");
  assert.equal(rows[1].digest.startsWith("sha256:"), true);
});

test("rollback confirm text mentions target revision", () => {
  const text = rollbackConfirmText({
    number: 3,
    shaShort: "ddd3333",
    digest: "sha256:ddd",
    state: "READY",
  });
  assert.match(text, /revision 3/);
  assert.match(text, /ddd3333/);
});

test("logs query formats source and tail", () => {
  assert.equal(logsQuery("build", 100), "?source=build&tail=100");
  assert.equal(logsQuery("app", 7.9), "?source=app&tail=7");
});
