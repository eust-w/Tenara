// Server-side masking (RB§22): secret values are replaced with a constant
// marker before any response reaches the browser.
export const MASKED_VALUE = "configured";

export function maskSecretValues(values: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const key of Object.keys(values)) {
    out[key] = MASKED_VALUE;
  }
  return out;
}

export function maskSecretPayload(payload: unknown): unknown {
  const p = payload as { values?: Record<string, unknown> } | null;
  if (p === null || typeof p !== "object" || p.values === undefined || p.values === null) {
    return payload;
  }
  return { ...p, values: maskSecretValues(p.values) };
}
