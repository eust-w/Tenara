// Shared route-guard predicate used by middleware (RB§9).
export const SESSION_COOKIE = "tenara_session";

const PUBLIC_PATHS = ["/login", "/register", "/verify", "/forgot"];

export function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"));
}

export function needsRedirect(pathname: string, hasSessionCookie: boolean): boolean {
  return !isPublicPath(pathname) && !hasSessionCookie;
}
