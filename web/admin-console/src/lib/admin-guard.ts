// Admin console route guard (RB§6 §30). The middleware only enforces that a
// session exists — the platform re-checks the platform_admin capability on
// every proxied call (server-side double check).
export const ADMIN_SESSION_COOKIE = "tenara_session";

const PUBLIC_PATHS = ["/login"];

export function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"));
}

export function needsRedirect(pathname: string, hasSessionCookie: boolean): boolean {
  return !isPublicPath(pathname) && !hasSessionCookie;
}
