import { expect, test } from "@playwright/test";

test("smoke: control plane healthz responds", async ({ request }) => {
  const res = await request.get("/healthz");
  expect(res.status()).toBe(200);
  // eslint-disable-next-line no-console
  console.log("PASS smoke");
});
