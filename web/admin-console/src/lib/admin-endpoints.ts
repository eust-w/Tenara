// Admin console resource -> control-plane endpoint mapping (RB§6 §30).
// cluster data only ever appears as pre-aggregated counters via the platform.

export type AdminResource = "users" | "apps" | "quota" | "cluster-health" | "security-events";

// Read-only resources; quota is write-only from the console perspective.
export type AdminReadResource = Exclude<AdminResource, "quota">;

const READ_PATHS: Record<AdminReadResource, string> = {
  users: "/v1/admin/users",
  apps: "/v1/admin/apps",
  "cluster-health": "/v1/admin/cluster/health",
  "security-events": "/v1/admin/security-events",
};

const WRITE_ACTIONS: Record<string, string> = {
  suspend: "/v1/admin/users/{id}/suspend",
  stop: "/v1/admin/apps/{id}/stop",
  quota: "/v1/admin/apps/{id}/quota",
};

export function isReadResource(value: string): value is AdminReadResource {
  return value in READ_PATHS;
}

export function readPath(resource: AdminReadResource): string {
  return READ_PATHS[resource];
}

export function isWriteAction(value: string): boolean {
  return value in WRITE_ACTIONS;
}

export function writePath(action: string, id: string): string {
  return WRITE_ACTIONS[action].replace("{id}", encodeURIComponent(id));
}

export function writeMethod(action: string): "PUT" | "POST" {
  return action === "quota" ? "PUT" : "POST";
}
