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

export async function GET(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const { id } = await ctx.params;
  const upstream = await fetch(`${API}/v1/apps/${id}/revisions`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const payload = await upstream.json().catch(() => ({ revisions: [] }));
  return NextResponse.json(payload, { status: upstream.status });
}
