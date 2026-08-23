import { NextResponse } from "next/server";

import { normalizeRepoList, onlyPickerFields } from "@/lib/repos";

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";

export async function GET(request: Request) {
  const session = request.headers
    .get("cookie")
    ?.split(";")
    .map((part) => part.trim().split("="))
    .find(([k]) => k === "tenara_session");
  const token = session?.[1];
  if (token === undefined || token === "") {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const url = new URL(request.url);
  const upstream = await fetch(`${API}/v1/integrations/github/repos`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const payload = (await upstream.json().catch(() => null)) as unknown;
  const repos = normalizeRepoList(payload).map(onlyPickerFields);
  return NextResponse.json(
    { repos, query: url.searchParams.get("q") ?? "" },
    { status: upstream.status },
  );
}
