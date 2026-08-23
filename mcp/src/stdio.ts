#!/usr/bin/env node
// Local stdio wrapper over the shared registry (D4). Fails fast when the
// platform connection is not configured; no business logic is duplicated.
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

import { TenaraClient } from "@tenara/sdk";

import { registerAllTools } from "./registerTools.ts";

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (value === undefined || value === "") {
    process.stderr.write(`tenara-mcp: env ${name} is required\n`);
    process.exit(1);
  }
  return value;
}

const apiURL = requiredEnv("TENARA_API_URL");
const token = requiredEnv("TENARA_API_TOKEN");

const server = new McpServer({ name: "tenara-mcp", version: "0.1.0" });
registerAllTools(server, {
  client: new TenaraClient({ baseURL: apiURL, token }),
  adminGate: {
    capabilityOf: async () => {
      // 平台裁决:能列 admin 用户即具备管理能力(本地 stdio 单用户语义)
      try {
        const resp = await fetch(`${apiURL.replace(/\/$/, "")}/v1/admin/users`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        return resp.status === 200;
      } catch {
        return false;
      }
    },
  },
  token,
});

await server.connect(new StdioServerTransport());
