import { NextResponse, type NextRequest } from "next/server";

import { needsRedirect, SESSION_COOKIE } from "@/lib/auth-guard";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasSession = request.cookies.get(SESSION_COOKIE) !== undefined;
  if (needsRedirect(pathname, hasSession)) {
    const url = request.nextUrl.clone();
    url.pathname = "/login";
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next|favicon.ico|api/).*)"],
};
