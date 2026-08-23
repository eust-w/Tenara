// Shared tool registry for both MCP transports (D4): the remote streamable
// HTTP gateway and the local stdio wrapper MUST expose identical sets.
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { TenaraClient } from "@tenara/sdk";

import type { AdminGate } from "./tools_admin.ts";
import { registerAdminTools } from "./tools_admin.ts";

export type { AdminGate };
import { registerAppTools } from "./tools.ts";

export interface ToolRegistryDeps {
  client: TenaraClient;
  adminGate: AdminGate;
  token: string;
}

// registerAllTools installs the liveness probe plus every app.* and admin.*
// definition onto a fresh server instance.
export function registerAllTools(server: McpServer, deps: ToolRegistryDeps): void {
  server.registerTool("ping", { description: "liveness probe tool" }, async () => ({
    content: [{ type: "text" as const, text: "pong" }],
  }));
  registerAppTools(server, deps.client);
  registerAdminTools(server, deps.client, deps.adminGate, deps.token);
}
