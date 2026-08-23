// Settings resource -> control-plane endpoint mapping (RB§22 RB§31).
// App-scoped resources require the app_id query; tokens/members resolve the
// org from the session token server-side.

export type SettingsResource = "domains" | "databases" | "secrets" | "tokens" | "members";

const APP_SCOPED: Record<"domains" | "databases" | "secrets", (appId: string) => string> = {
  domains: (appId) => `/v1/apps/${appId}/domains`,
  databases: (appId) => `/v1/apps/${appId}/databases`,
  secrets: (appId) => `/v1/apps/${appId}/env`,
};

const ORG_SCOPED: Record<"tokens" | "members", string> = {
  tokens: "/v1/org/tokens",
  members: "/v1/org/members",
};

export function isSettingsResource(value: string): value is SettingsResource {
  return value in APP_SCOPED || value in ORG_SCOPED;
}

export function listPath(resource: SettingsResource, appId: string | null): string {
  const scoped = APP_SCOPED[resource as keyof typeof APP_SCOPED];
  if (typeof scoped === "function") {
    return scoped(appId ?? "");
  }
  return ORG_SCOPED[resource as "tokens" | "members"];
}

export function isAppScoped(resource: SettingsResource): boolean {
  return resource in APP_SCOPED;
}

export function writePath(resource: SettingsResource, appId: string | null): string {
  if (isAppScoped(resource)) {
    if (resource === "secrets") {
      return APP_SCOPED.secrets(appId ?? "");
    }
    return APP_SCOPED[resource as "domains" | "databases"](appId ?? "");
  }
  return ORG_SCOPED[resource as "tokens" | "members"];
}
