import { NextResponse } from "next/server";

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";

export async function POST(request: Request) {
  const body = await request.json().catch(() => null);
  if (body === null || typeof body !== "object") {
    return NextResponse.json({ error: "invalid json" }, { status: 400 });
  }
  const upstream = await fetch(`${API}/v1/auth/login`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  const payload = (await upstream.json().catch(() => ({}))) as { token?: unknown };
  if (upstream.status !== 200 || typeof payload.token !== "string") {
    return NextResponse.json({ error: "login failed" }, { status: upstream.status });
  }
  const res = NextResponse.json({ ok: true });
  res.cookies.set("tenara_session", payload.token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: 60 * 60 * 12,
  });
  return res;
}
