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

// Analyze uses the app's registered repository, resolved server-side from the
// app record so the browser never has to carry it around.
export async function POST(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const token = bearer(request);
  if (token === null) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const { id } = await ctx.params;

  const detailRes = await fetch(`${API}/v1/apps/${id}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!detailRes.ok) {
    return NextResponse.json({ error: "app lookup failed" }, { status: detailRes.status });
  }
  const detail = (await detailRes.json().catch(() => ({}))) as {
    repo_url?: string;
    repo?: string;
  };
  const repoURL = detail.repo_url ?? detail.repo ?? "";
  if (repoURL === "") {
    return NextResponse.json({ error: "app has no repository bound" }, { status: 409 });
  }

  const upstream = await fetch(`${API}/v1/analyze`, {
    method: "POST",
    headers: { "content-type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ repo_url: repoURL }),
  });
  const payload = await upstream.json().catch(() => ({}));
  return NextResponse.json(payload, { status: upstream.status });
}
