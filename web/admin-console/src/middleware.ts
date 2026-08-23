import { NextResponse, type NextRequest } from "next/server";

import { needsRedirect, ADMIN_SESSION_COOKIE } from "@/lib/admin-guard";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasSession = request.cookies.get(ADMIN_SESSION_COOKIE) !== undefined;
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
