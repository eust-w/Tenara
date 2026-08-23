// Remote MCP gateway: streamable HTTP transport behind bearer auth with
// DNS-rebinding guards (D4, RB§8). A fresh McpServer instance is built for
// every request — tool registration never lives on a shared singleton.
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import type { IncomingMessage, ServerResponse } from "node:http";

import { TenaraClient } from "@tenara/sdk";

import { registerAllTools, type AdminGate } from "./registerTools.ts";

type PlatformFetch = (
  url: string,
  init?: { headers?: Record<string, string> },
) => Promise<{ status: number }>;

const globalScope = globalThis as unknown as { fetch?: PlatformFetch };

export interface GatewayDeps {
  platformURL: string;
  validatePath?: string;
  allowedHosts: string[];
  allowedOrigins: string[];
  fetchImpl?: PlatformFetch;
  client?: TenaraClient;
  adminGate?: AdminGate;
  token?: string;
}

// ---- pure guards (DNS-rebinding defence, §53.1) ----

export function hostAllowed(hostHeader: string | undefined, allowedHosts: string[]): boolean {
  if (hostHeader === undefined) {
    return false;
  }
  if (allowedHosts.includes(hostHeader)) {
    return true;
  }
  // Host headers usually carry the port; fall back to the bare hostname.
  try {
    const hostname = new URL(`http://${hostHeader}`).hostname;
    return allowedHosts.includes(hostname);
  } catch {
    return false;
  }
}

// Absence of Origin means a non-browser client (curl / MCP SDK): rebinding is
// a browser-only attack, so a missing header passes the gate.
export function originAllowed(
  origin: string | undefined,
  allowedOrigins: string[],
  allowedHosts: string[],
): boolean {
  if (origin === undefined) {
    return true;
  }
  try {
    const parsed = new URL(origin);
    if (allowedOrigins.includes(origin)) {
      return true;
    }
    return allowedHosts.includes(parsed.host);
  } catch {
    return false;
  }
}

// ---- bearer validation against the platform API ----

async function validateToken(deps: GatewayDeps, req: IncomingMessage): Promise<boolean> {
  const auth = req.headers.authorization ?? "";
  if (!auth.startsWith("Bearer ")) {
    return false;
  }
  const token = auth.slice("Bearer ".length);
  const target = `${deps.platformURL.replace(/\/$/, "")}${deps.validatePath ?? "/v1/orgs"}`;
  const doFetch = deps.fetchImpl ?? globalScope.fetch;
  if (doFetch === undefined) {
    return false;
  }
  try {
    const resp = await doFetch(target, { headers: { Authorization: `Bearer ${token}` } });
    return resp.status === 200;
  } catch {
    return false;
  }
}

// ---- per-request server factory (never a shared singleton, §53.1) ----

// Per-request factory (§53.1): a fresh server sharing the common registry.
export function buildServer(deps: GatewayDeps): McpServer {
  const server = new McpServer({ name: "tenara-mcp", version: "0.1.0" });
  registerAllTools(server, {
    client:
      deps.client ??
      new TenaraClient({
        baseURL: deps.platformURL,
        token: deps.token ?? "",
      }),
    adminGate: deps.adminGate ?? { capabilityOf: async () => true },
    token: deps.token ?? "",
  });
  return server;
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = "";
    req.setEncoding("utf8");
    req.on("data", (chunk: string) => {
      data += chunk;
    });
    req.on("end", () => {
      resolve(data);
    });
    req.on("error", reject);
  });
}

function json(res: ServerResponse, status: number, payload: unknown): void {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(payload));
}

async function handleMcpPost(
  deps: GatewayDeps,
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  // DNS-rebinding guards run before authentication.
  if (!hostAllowed(req.headers.host, deps.allowedHosts)) {
    json(res, 403, { error: "host not allowed" });
    return;
  }
  const origin = req.headers.origin;
  if (!originAllowed(origin, deps.allowedOrigins, deps.allowedHosts)) {
    json(res, 403, { error: "origin not allowed" });
    return;
  }
  if (!(await validateToken(deps, req))) {
    json(res, 401, { error: "missing or invalid bearer token" });
    return;
  }

  const raw = await readBody(req);
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    json(res, 400, { error: "invalid json body" });
    return;
  }

  // Stateless mode: omitting sessionIdGenerator pairs with the per-request
  // factory so no session state survives between requests.
  const transport = new StreamableHTTPServerTransport({});
  const server = buildServer(deps);
  await server.connect(transport as Parameters<typeof server.connect>[0]);
  await transport.handleRequest(req, res, parsed);
}

// createGateway wires the health endpoint, the /mcp endpoint and a JSON 404
// fallback into a single Node request listener.
export function createGateway(
  deps: GatewayDeps,
): (req: IncomingMessage, res: ServerResponse) => Promise<void> {
  return async (req, res) => {
    const path = (req.url ?? "").split("?")[0];
    if (req.method === "GET" && path === "/healthz") {
      res.writeHead(200, { "content-type": "text/plain" });
      res.end("ok");
      return;
    }
    if (req.method === "POST" && path === "/mcp") {
      await handleMcpPost(deps, req, res);
      return;
    }
    json(res, 404, { error: "not found" });
  };
}
