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

// Logs are scoped by the platform using the session token's org — the BFF is
// a transparent passthrough and never widens access.
export async function GET(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const { id } = await ctx.params;
  const query = new URL(request.url).search;
  const upstream = await fetch(`${API}/v1/apps/${id}/logs${query}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const text = await upstream.text();
  return new Response(text, {
    status: upstream.status,
    headers: { "content-type": upstream.headers.get("content-type") ?? "text/plain" },
  });
}
