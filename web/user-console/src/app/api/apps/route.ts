import { NextResponse } from "next/server";

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";

function bearer(request: Request): string | null {
  const cookie = request.headers
    .get("cookie")
    ?.split(";")
    .map((part) => part.trim().split("="))
    .find(([k]) => k === "tenara_session");
  return cookie?.[1] ?? null;
}

function authHeaders(request: Request): Record<string, string> {
  const token = bearer(request);
  if (token === null) {
    throw new Error("unauthenticated");
  }
  return { "content-type": "application/json", Authorization: `Bearer ${token}` };
}

export async function GET(request: Request) {
  let headers: Record<string, string>;
  try {
    headers = authHeaders(request);
  } catch {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const upstream = await fetch(`${API}/v1/apps`, { headers });
  const payload = await upstream.json().catch(() => ({ apps: [] }));
  return NextResponse.json(payload, { status: upstream.status });
}

export async function POST(request: Request) {
  let headers: Record<string, string>;
  try {
    headers = authHeaders(request);
  } catch {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const body = await request.json().catch(() => null);
  if (body === null || typeof body !== "object") {
    return NextResponse.json({ error: "invalid json" }, { status: 400 });
  }
  const upstream = await fetch(`${API}/v1/apps`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
  const payload = await upstream.json().catch(() => ({}));
  return NextResponse.json(payload, { status: upstream.status });
}
