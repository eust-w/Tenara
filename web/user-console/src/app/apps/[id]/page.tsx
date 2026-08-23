"use client";

import { useParams, useRouter } from "next/navigation";
import { useState } from "react";

import {
  canDeploy,
  isUnsupportedStack,
  supportedRuntimes,
  type PlanSummary,
} from "@/lib/plan-view";

export default function AppApprovalPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const appId = params.id;

  const [analysis, setAnalysis] = useState<unknown>(null);
  const [analyzed, setAnalyzed] = useState(false);
  const [plan, setPlan] = useState<PlanSummary | null>(null);
  const [unsupported, setUnsupported] = useState<string[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function postJSON(path: string): Promise<unknown> {
    const res = await fetch(path, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: "{}",
    });
    return (await res.json().catch(() => ({}))) as unknown;
  }

  async function runAnalyze() {
    setBusy(true);
    setError(null);
    try {
      const body = await postJSON(`/api/apps/${appId}/analyze`);
      setAnalysis(body);
      setAnalyzed(true);
      if (isUnsupportedStack(body)) {
        setUnsupported(supportedRuntimes(body));
        setPlan(null);
      } else {
        setUnsupported(null);
      }
    } finally {
      setBusy(false);
    }
  }

  async function generatePlan() {
    setBusy(true);
    setError(null);
    try {
      const body = (await postJSON(`/api/apps/${appId}/plan`)) as PlanSummary & {
        code?: string;
        unsupported?: { supported?: string[] };
        id?: string;
        state?: string;
      };
      if (isUnsupportedStack(body)) {
        setUnsupported(supportedRuntimes(body));
        setPlan(null);
      } else if (typeof body.id === "string" && typeof body.state === "string") {
        setPlan({
          id: body.id,
          state: body.state,
          namespace: body.namespace,
          domain: body.domain,
          services: [],
        });
      } else {
        setError("plan generation failed");
      }
    } finally {
      setBusy(false);
    }
  }

  async function approve() {
    if (plan === null || !canDeploy(plan)) {
      return;
    }
    setBusy(true);
    const res = await fetch(`/api/apps/${appId}/deploy`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ plan_id: plan.id }),
    });
    setBusy(false);
    if (res.ok) {
      router.push(`/apps/${appId}`);
    } else {
      setError("deployment rejected");
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">App {appId}</h1>

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">1 · Analyze repository</h2>
        <button
          type="button"
          onClick={() => void runAnalyze()}
          disabled={busy}
          className="rounded bg-neutral-800 px-3 py-2 hover:border-emerald-500 disabled:opacity-50"
        >
          Run analysis
        </button>
        {analyzed && (
          <pre className="overflow-x-auto rounded bg-neutral-900 p-3 text-xs">
            {JSON.stringify(analysis, null, 2)}
          </pre>
        )}
      </section>

      {unsupported !== null && (
        <section className="space-y-1 rounded border border-red-500 p-4 text-red-400">
          <h2 className="font-medium">UNSUPPORTED_STACK</h2>
          <p>supported runtimes: {unsupported.join(", ") || "(none listed)"}</p>
        </section>
      )}

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">2 · Plan approval (R9)</h2>
        <button
          type="button"
          onClick={() => void generatePlan()}
          disabled={busy || !analyzed}
          className="rounded bg-neutral-800 px-3 py-2 hover:border-emerald-500 disabled:opacity-50"
        >
          Generate plan
        </button>
        {plan !== null && (
          <dl className="text-sm text-neutral-300">
            <div>namespace: {plan.namespace ?? "-"}</div>
            <div>domain: {plan.domain ?? "-"}</div>
            <div>services: {(plan.services ?? []).length}</div>
          </dl>
        )}
      </section>

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">3 · Deploy</h2>
        <button
          type="button"
          onClick={() => void approve()}
          disabled={busy || !canDeploy(plan)}
          className="rounded bg-emerald-500 px-3 py-2 font-medium text-black hover:bg-emerald-400 disabled:opacity-50"
        >
          Approve &amp; deploy
        </button>
        <p className="text-xs text-neutral-500">
          Approve only appears for a PLANNED plan — the plan itself is the approval.
        </p>
      </section>

      {error !== null && <p className="text-sm text-red-400">{error}</p>}
    </div>
  );
}
