// Plan approval view helpers (RB§11 R8 R9).

export interface PlanSummary {
  id?: string;
  state?: string;
  namespace?: string;
  domain?: string;
  services?: Array<{ name: string; image?: string; port?: number }>;
}

// R8: unsupported stacks must surface the supported runtime list.
export function isUnsupportedStack(payload: unknown): boolean {
  const p = payload as { code?: string; unsupported?: { code?: string } };
  return p?.code === "UNSUPPORTED_STACK" || p?.unsupported?.code === "UNSUPPORTED_STACK";
}

export function supportedRuntimes(payload: unknown): string[] {
  const p = payload as { supported?: string[]; unsupported?: { supported?: string[] } };
  if (Array.isArray(p?.unsupported?.supported)) {
    return p.unsupported.supported;
  }
  if (Array.isArray(p?.supported)) {
    return p.supported;
  }
  return [];
}

// R9: the deploy button is only an approval for a PLANNED plan; without one
// it must never render.
export function canDeploy(plan: { id?: string; state?: string } | null | undefined): boolean {
  return (
    plan !== null && plan !== undefined && typeof plan.id === "string" && plan.state === "PLANNED"
  );
}

export function failedStepsFromReport(payload: unknown): number[] {
  const steps = (payload as { steps?: Array<{ id?: number; status?: string }> })?.steps ?? [];
  return steps
    .filter((s) => s.status === "fail" && typeof s.id === "number")
    .map((s) => s.id as number);
}
