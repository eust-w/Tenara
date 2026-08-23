import { NextResponse } from "next/server";

const API = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";

// Redirect to the control-plane OAuth initiation; the GitHub token never
// travels through the browser.
export async function GET() {
  return NextResponse.redirect(`${API}/v1/auth/github`);
}
