// Admin tool surface with capability gating (RB§8 admin list, RB§31). Every
// call is checked against an injected capability oracle before any network
// activity; cluster data only ever appears as pre-aggregated counters.
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";

import type { TenaraClient } from "@tenara/sdk";

import { dualContent, type ToolOut } from "./tools.ts";

export interface AdminGate {
  capabilityOf: (token: string, capability: string) => Promise<boolean>;
}

const CAP_USERS = "admin.users";
const CAP_APPS = "admin.apps";
const CAP_QUOTA = "admin.quota";
const CAP_CLUSTER_READ = "admin.cluster.read";
const CAP_SECURITY_READ = "admin.security.read";

interface AdminToolDef {
  name: string;
  capability: string;
  description: string;
  method: string;
  pathTemplate: (args: Record<string, string>) => string;
  shape: z.ZodRawShape;
}

const userIdField = { user_id: z.string().min(1) };
const appIdField = { app_id: z.string().min(1) };

export const ADMIN_TOOLS: AdminToolDef[] = [
  {
    name: "admin.user.list",
    capability: CAP_USERS,
    description: "list platform users with roles and suspension state",
    method: "GET",
    pathTemplate: () => "/v1/admin/users",
    shape: {},
  },
  {
    name: "admin.user.suspend",
    capability: CAP_USERS,
    description: "suspend a user account; all their tokens stop working",
    method: "POST",
    pathTemplate: (args) => `/v1/admin/users/${args["user_id"]}/suspend`,
    shape: userIdField,
  },
  {
    name: "admin.app.list",
    capability: CAP_APPS,
    description: "list every application across orgs",
    method: "GET",
    pathTemplate: () => "/v1/admin/apps",
    shape: {},
  },
  {
    name: "admin.app.stop",
    capability: CAP_APPS,
    description: "force-stop an application workload",
    method: "POST",
    pathTemplate: (args) => `/v1/admin/apps/${args["app_id"]}/stop`,
    shape: appIdField,
  },
  {
    name: "admin.quota.set",
    capability: CAP_QUOTA,
    description: "set the resource tier of an application",
    method: "PUT",
    pathTemplate: (args) => `/v1/admin/apps/${args["app_id"]}/quota`,
    shape: { ...appIdField, tier: z.enum(["free", "pro"]) },
  },
  {
    name: "admin.cluster.health",
    capability: CAP_CLUSTER_READ,
    description: "aggregated cluster health: ready node count and pod restart totals",
    method: "GET",
    pathTemplate: () => "/v1/admin/cluster/health",
    shape: {},
  },
  {
    name: "admin.security.events",
    capability: CAP_SECURITY_READ,
    description: "recent security events across the platform",
    method: "GET",
    pathTemplate: () => "/v1/admin/security-events",
    shape: {},
  },
];

function jsonBodyFor(def: AdminToolDef, args: Record<string, string>): unknown {
  if (def.name === "admin.quota.set") {
    return { tier: args["tier"] };
  }
  return undefined;
}

async function runAllowed(
  def: AdminToolDef,
  args: Record<string, string>,
  client: TenaraClient,
): Promise<ToolOut> {
  const target = def.pathTemplate(args);
  const body = jsonBodyFor(def, args);
  let res: { status: number; data: unknown };
  if (def.method === "GET") {
    res = await client.get(target);
  } else if (def.method === "PUT") {
    res = await client.put(target, body);
  } else {
    res = await client.post(target, body);
  }
  return { human: `${def.name} ok`, data: res.data };
}

// runAdminChecked resolves the capability gate first; a denial returns a
// forbidden marker before any request leaves the process.
export async function runAdminChecked(
  def: AdminToolDef,
  args: Record<string, string>,
  client: TenaraClient,
  gate: AdminGate,
  token: string,
): Promise<ToolOut> {
  if (!(await gate.capabilityOf(token, def.capability))) {
    return {
      human: `forbidden: missing capability ${def.capability}`,
      data: { error: "forbidden", capability: def.capability },
      failed: true,
    };
  }
  return runAllowed(def, args, client);
}

// registerAdminTools wires the gated admin surface onto an MCP server.
export function registerAdminTools(
  server: McpServer,
  client: TenaraClient,
  gate: AdminGate,
  token: string,
): void {
  for (const def of ADMIN_TOOLS) {
    server.registerTool(
      def.name,
      {
        description: `[platform_admin] ${def.description}`,
        inputSchema: def.shape,
      },
      async (rawArgs: unknown) => {
        const args = (rawArgs ?? {}) as Record<string, string>;
        const out = await runAdminChecked(def, args, client, gate, token);
        const result = dualContent(out.human, out.data) as {
          content: Array<{ type: "text"; text: string }>;
          isError?: boolean;
        };
        result.isError = out.failed === true;
        return result;
      },
    );
  }
}
