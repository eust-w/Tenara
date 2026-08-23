import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./scenarios",
  timeout: 30_000,
  retries: 0,
  reporter: [["list"]],
  use: {
    baseURL: process.env.TENARA_API_URL ?? "http://127.0.0.1:18080",
    trace: "off",
  },
});
