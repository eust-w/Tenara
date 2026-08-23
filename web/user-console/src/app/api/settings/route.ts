import { NextResponse } from "next/server";

import {
  isAppScoped,
  isSettingsResource,
  listPath,
  writePath,
  type SettingsResource,
} from "@/lib/settings-endpoints";
import { maskSecretPayload } from "@/lib/settings-mask";

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";

function bearer(request: Request): string | null {
  const cookie = request.headers
    .get("cookie")
    ?.split(";")
    .map((part) => part.trim().split("="))
    .find(([k]) => k === "tenara_session");
  return cookie?.[1] ?? null;
}

function requireScope(resource: SettingsResource, appId: string | null): NextResponse | null {
  if (isAppScoped(resource) && (appId === null || appId === "")) {
    return NextResponse.json({ error: "app_id required for this resource" }, { status: 400 });
  }
  return null;
}

export async function GET(request: Request) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const url = new URL(request.url);
  const resource = url.searchParams.get("resource") ?? "";
  if (!isSettingsResource(resource)) {
    return NextResponse.json({ error: "unknown resource" }, { status: 400 });
  }
  const appId = url.searchParams.get("app_id");
  const scopeError = requireScope(resource, appId);
  if (scopeError !== null) {
    return scopeError;
  }
  const upstream = await fetch(`${API}${listPath(resource, appId)}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  let payload: unknown = await upstream.json().catch(() => ({}));
  if (resource === "secrets") {
    payload = maskSecretPayload(payload);
  }
  return NextResponse.json(payload, { status: upstream.status });
}

interface SettingsPostBody {
  resource?: string;
  app_id?: string;
  action?: string;
  payload?: Record<string, unknown>;
}

export async function POST(request: Request) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const body = (await request.json().catch(() => null)) as SettingsPostBody | null;
  if (body === null || typeof body.resource !== "string" || !isSettingsResource(body.resource)) {
    return NextResponse.json({ error: "unknown resource" }, { status: 400 });
  }
  const resource: SettingsResource = body.resource;
  const appId = typeof body.app_id === "string" ? body.app_id : null;
  const scopeError = requireScope(resource, appId);
  if (scopeError !== null) {
    return scopeError;
  }

  const target = writePath(resource, appId);
  const method = resource === "secrets" ? "PUT" : "POST";
  const upstream = await fetch(`${API}${target}`, {
    method,
    headers: { "content-type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify(body.payload ?? {}),
  });
  let data: unknown = await upstream.json().catch(() => ({}));
  if (resource === "secrets") {
    data = maskSecretPayload(data);
  }
  return NextResponse.json(data, { status: upstream.status });
}
