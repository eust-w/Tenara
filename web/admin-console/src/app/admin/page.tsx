"use client";

import { useCallback, useEffect, useState } from "react";

type Section = "users" | "apps" | "cluster-health" | "security-events";

const SECTIONS: Array<{ resource: Section; label: string }> = [
  { resource: "users", label: "Users" },
  { resource: "apps", label: "Apps" },
  { resource: "cluster-health", label: "Cluster health" },
  { resource: "security-events", label: "Security events" },
];

export default function AdminPage() {
  const [data, setData] = useState<Partial<Record<Section, unknown>>>({});
  const [quotaAppId, setQuotaAppId] = useState("");
  const [tier, setTier] = useState("pro");
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (resource: Section) => {
    const res = await fetch(`/api/admin?resource=${resource}`);
    if (res.status === 403) {
      setError("forbidden — platform_admin capability required");
      return;
    }
    if (res.ok) {
      const body: unknown = await res.json();
      setData((prev) => ({ ...prev, [resource]: body }));
    }
  }, []);

  const refreshAll = useCallback(() => {
    for (const section of SECTIONS) {
      void load(section.resource);
    }
  }, [load]);

  useEffect(() => {
    refreshAll();
  }, [refreshAll]);

  async function act(action: string, targetId: string, quotaTier?: string) {
    setError(null);
    const res = await fetch("/api/admin", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ action, target_id: targetId, tier: quotaTier }),
    });
    if (!res.ok) {
      setError(`${action} failed (${res.status})`);
      return;
    }
    refreshAll();
  }

  function asRows(payload: unknown): Array<Record<string, unknown>> {
    return Array.isArray(payload) ? payload : [];
  }

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-semibold">Platform administration</h1>
      {error !== null && <p className="text-sm text-red-400">{error}</p>}

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">Users</h2>
        <ul className="space-y-1 text-sm">
          {asRows(data.users).map((u) => (
            <li key={String(u.id ?? u.email)} className="flex items-center justify-between gap-3">
              <span>{String(u.email ?? u.id)}</span>
              <button
                type="button"
                onClick={() => void act("suspend", String(u.id ?? u.email))}
                className="rounded border px-2 py-0.5 hover:border-red-400"
              >
                Suspend
              </button>
            </li>
          ))}
        </ul>
      </section>

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">Apps</h2>
        <ul className="space-y-1 text-sm">
          {asRows(data.apps).map((a) => (
            <li key={String(a.id ?? a.name)} className="flex items-center justify-between gap-3">
              <span>
                {String(a.name ?? a.id)} ·{" "}
                <span className="text-neutral-400">{String(a.state ?? "")}</span>
              </span>
              <button
                type="button"
                onClick={() => void act("stop", String(a.id ?? a.name))}
                className="rounded border px-2 py-0.5 hover:border-red-400"
              >
                Stop
              </button>
            </li>
          ))}
        </ul>
      </section>

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">Quota override</h2>
        <div className="flex items-center gap-2 text-sm">
          <input
            value={quotaAppId}
            onChange={(e) => setQuotaAppId(e.target.value)}
            placeholder="app id"
            className="rounded border border-neutral-700 bg-neutral-900 px-2 py-1"
          />
          <select
            value={tier}
            onChange={(e) => setTier(e.target.value)}
            className="rounded border border-neutral-700 bg-neutral-900 px-2 py-1"
          >
            <option value="free">free</option>
            <option value="pro">pro</option>
          </select>
          <button
            type="button"
            onClick={() => void act("quota", quotaAppId, tier)}
            className="rounded border px-2 py-1 hover:border-emerald-500"
          >
            Set tier
          </button>
        </div>
      </section>

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">Cluster health</h2>
        <pre className="max-h-48 overflow-auto rounded bg-neutral-900 p-3 text-xs">
          {JSON.stringify(data["cluster-health"] ?? null, null, 2)}
        </pre>
      </section>

      <section className="space-y-2 rounded border border-neutral-700 p-4">
        <h2 className="font-medium">Security events</h2>
        <pre className="max-h-64 overflow-auto rounded bg-neutral-900 p-3 text-xs">
          {JSON.stringify(data["security-events"] ?? [], null, 2)}
        </pre>
      </section>
    </div>
  );
}
