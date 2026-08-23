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

export async function POST(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const { id } = await ctx.params;
  const body = (await request.json().catch(() => ({}))) as { plan_id?: string };
  if (typeof body.plan_id !== "string" || body.plan_id === "") {
    return NextResponse.json({ error: "plan_id is required" }, { status: 400 });
  }
  const upstream = await fetch(`${API}/v1/apps/${id}/deploy`, {
    method: "POST",
    headers: { "content-type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ plan_id: body.plan_id }),
  });
  const payload = await upstream.json().catch(() => ({}));
  return NextResponse.json(payload, { status: upstream.status });
}
