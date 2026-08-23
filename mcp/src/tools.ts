// Twelve high-level app tools exposed over MCP (RB§8, R2/R9/R11). Handlers
// speak platform REST only — no Kubernetes primitive is ever accepted or
// forwarded from tool arguments.
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";

import { TenaraClient } from "@tenara/sdk";

export interface ToolOut {
  human: string;
  data: unknown;
  failed?: boolean;
}

export interface ToolDef {
  name: string;
  description: string;
  shape: z.ZodRawShape;
  run: (rawArgs: unknown, client: TenaraClient) => Promise<ToolOut>;
}

// dualContent renders a tool result as one human-readable text block plus one
// structured JSON text block.
export function dualContent(
  human: string,
  data: unknown,
): { content: Array<{ type: "text"; text: string }> } {
  return {
    content: [
      { type: "text", text: human },
      { type: "text", text: JSON.stringify(data, null, 2) },
    ],
  };
}

const appIdField = { app_id: z.string().min(1).describe("platform application id") };

const createSchema = z.object({
  name: z.string().min(1),
  env: z.string().default("prod"),
});
const analyzeSchema = z.object({ repo_url: z.string().url() });
const planSchema = z.object({ ...appIdField });
const deploySchema = z.object({ ...appIdField, plan_id: z.string().min(1) });
const statusSchema = z.object({ ...appIdField });
const logsSchema = z.object({
  ...appIdField,
  tail: z.number().int().positive().max(1000).default(200),
});
const restartSchema = z.object({ ...appIdField });
const rollbackSchema = z.object({
  ...appIdField,
  target_revision: z.number().int().positive().optional(),
});
const deleteSchema = z.object({ ...appIdField });
const setEnvSchema = z.object({
  ...appIdField,
  values: z.record(z.string(), z.string()),
});
const dbSchema = z.object({ ...appIdField, kind: z.enum(["mongo", "redis", "storage"]) });
const domainSchema = z.object({ ...appIdField, hostname: z.string().min(1) });

export const APP_TOOLS: ToolDef[] = [
  {
    name: "app.create",
    description: "register a new application",
    shape: createSchema.shape,
    run: async (raw, client) => {
      const args = createSchema.parse(raw);
      await client.post("/v1/apps", { name: args.name, env: args.env });
      return { human: `created app ${args.name}`, data: { env: args.env } };
    },
  },
  {
    name: "app.analyze",
    description: "scan a repository and produce an AppSpec draft",
    shape: analyzeSchema.shape,
    run: async (raw, client) => {
      const args = analyzeSchema.parse(raw);
      const res = await client.post("/v1/analyze", { repo_url: args.repo_url });
      return { human: "analysis finished", data: res.data };
    },
  },
  {
    name: "app.plan",
    description: "generate the deployment plan for an app",
    shape: planSchema.shape,
    run: async (raw, client) => {
      const args = planSchema.parse(raw);
      const res = await client.post(`/v1/apps/${args.app_id}/plan`, {});
      return { human: "plan generated", data: res.data };
    },
  },
  {
    name: "app.deploy",
    description: "deploy the approved plan (plan approval is implicit, R9)",
    shape: deploySchema.shape,
    run: async (raw, client) => {
      const args = deploySchema.parse(raw);
      const res = await client.post(`/v1/apps/${args.app_id}/deploy`, {
        plan_id: args.plan_id,
      });
      return { human: "deployment started", data: res.data };
    },
  },
  {
    name: "app.status",
    description: "current state, revision and diagnostics reference",
    shape: statusSchema.shape,
    run: async (raw, client) => {
      const args = statusSchema.parse(raw);
      const res = await client.get(`/v1/apps/${args.app_id}`);
      return { human: "current status", data: res.data };
    },
  },
  {
    name: "app.logs",
    description: "tail runtime logs for the app",
    shape: logsSchema.shape,
    run: async (raw, client) => {
      const args = logsSchema.parse(raw);
      const res = await client.get(`/v1/apps/${args.app_id}/logs?tail=${args.tail}`);
      return { human: `last ${args.tail} log lines`, data: res.data };
    },
  },
  {
    name: "app.restart",
    description: "rolling restart of the workload",
    shape: restartSchema.shape,
    run: async (raw, client) => {
      const args = restartSchema.parse(raw);
      const res = await client.post(`/v1/apps/${args.app_id}/restart`, {});
      return { human: "restart triggered", data: res.data };
    },
  },
  {
    name: "app.rollback",
    description: "roll back to a previous revision (default: latest ready)",
    shape: rollbackSchema.shape,
    run: async (raw, client) => {
      const args = rollbackSchema.parse(raw);
      const payload =
        args.target_revision === undefined ? {} : { target_revision: args.target_revision };
      const res = await client.post(`/v1/apps/${args.app_id}/rollback`, payload);
      return { human: "rollback started", data: res.data };
    },
  },
  {
    name: "app.delete",
    description: "start the soft-delete pipeline for the app",
    shape: deleteSchema.shape,
    run: async (raw, client) => {
      const args = deleteSchema.parse(raw);
      const res = await client.del(`/v1/apps/${args.app_id}`);
      return { human: "deletion pipeline started", data: res.data };
    },
  },
  {
    name: "app.set_env",
    description: "store environment secrets; values are write-only and never echoed back",
    shape: setEnvSchema.shape,
    run: async (raw, client) => {
      const args = setEnvSchema.parse(raw);
      await client.put(`/v1/apps/${args.app_id}/env`, { values: args.values });
      const keys = Object.keys(args.values);
      return {
        human: keys.map((k) => `${k}: configured`).join("\n"),
        data: { values: Object.fromEntries(keys.map((k) => [k, "configured"])) },
      };
    },
  },
  {
    name: "database.create",
    description: "provision a scoped database/cache/storage binding",
    shape: dbSchema.shape,
    run: async (raw, client) => {
      const args = dbSchema.parse(raw);
      const res = await client.post(`/v1/apps/${args.app_id}/databases`, { kind: args.kind });
      return { human: `binding ${args.kind} provisioning`, data: res.data };
    },
  },
  {
    name: "domain.bind",
    description: "attach a custom hostname to the app",
    shape: domainSchema.shape,
    run: async (raw, client) => {
      const args = domainSchema.parse(raw);
      const res = await client.post(`/v1/apps/${args.app_id}/domains`, {
        hostname: args.hostname,
      });
      return { human: `domain ${args.hostname} queued`, data: res.data };
    },
  },
];

// registerAppTools wires every definition onto an MCP server instance using
// the shared platform client.
export function registerAppTools(server: McpServer, client: TenaraClient): void {
  for (const def of APP_TOOLS) {
    server.registerTool(
      def.name,
      { description: def.description, inputSchema: def.shape },
      async (rawArgs: unknown) => {
        const out = await def.run(rawArgs, client);
        return dualContent(out.human, out.data);
      },
    );
  }
}
