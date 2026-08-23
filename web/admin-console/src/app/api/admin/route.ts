import { NextResponse } from "next/server";

import {
  isReadResource,
  isWriteAction,
  readPath,
  writeMethod,
  writePath,
} from "@/lib/admin-endpoints";

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";

function bearer(request: Request): string | null {
  const cookie = request.headers
    .get("cookie")
    ?.split(";")
    .map((part) => part.trim().split("="))
    .find(([k]) => k === "tenara_session");
  return cookie?.[1] ?? null;
}

// Server-side double check (RB§6 §30): every proxied admin call first probes
// a platform_admin-only endpoint; any non-200 verdict short-circuits to 403
// so the browser never sees admin payloads without the capability.
async function assertPlatformAdmin(request: Request, token: string): Promise<boolean> {
  const upstream = await fetch(`${API}/v1/admin/users`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return upstream.status === 200;
}

export async function GET(request: Request) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const url = new URL(request.url);
  const resource = url.searchParams.get("resource") ?? "";
  if (!isReadResource(resource)) {
    return NextResponse.json({ error: "unknown resource" }, { status: 400 });
  }
  if (!(await assertPlatformAdmin(request, token))) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }
  const upstream = await fetch(`${API}${readPath(resource)}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const payload = await upstream.json().catch(() => ({}));
  return NextResponse.json(payload, { status: upstream.status });
}

interface AdminPostBody {
  action?: string;
  target_id?: string;
  tier?: string;
}

export async function POST(request: Request) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const body = (await request.json().catch(() => null)) as AdminPostBody | null;
  if (body === null || typeof body.action !== "string" || !isWriteAction(body.action)) {
    return NextResponse.json({ error: "unknown action" }, { status: 400 });
  }
  if (!(await assertPlatformAdmin(request, token))) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }
  const targetId = typeof body.target_id === "string" ? body.target_id : "";
  const target = writePath(body.action, targetId);
  const payload = body.action === "quota" ? { tier: body.tier ?? "" } : {};
  const upstream = await fetch(`${API}${target}`, {
    method: writeMethod(body.action),
    headers: { "content-type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify(payload),
  });
  const data = await upstream.json().catch(() => ({}));
  return NextResponse.json(data, { status: upstream.status });
}
