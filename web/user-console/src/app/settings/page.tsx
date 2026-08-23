"use client";

import { useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";

type Resource = "domains" | "databases" | "secrets" | "tokens" | "members";

const SECTIONS: Array<{ resource: Resource; label: string; appScoped: boolean }> = [
  { resource: "domains", label: "Domains", appScoped: true },
  { resource: "databases", label: "Databases", appScoped: true },
  { resource: "secrets", label: "Secrets (values shown as configured)", appScoped: true },
  { resource: "tokens", label: "API tokens", appScoped: false },
  { resource: "members", label: "Members", appScoped: false },
];

function SettingsInner() {
  const searchParams = useSearchParams();
  const appId = searchParams.get("app_id") ?? "";
  const [lists, setLists] = useState<Partial<Record<Resource, unknown>>>({});
  const [draft, setDraft] = useState<Partial<Record<Resource, string>>>({});

  const load = useCallback(
    async (resource: Resource) => {
      if (appId === "" && (SECTIONS.find((s) => s.resource === resource)?.appScoped ?? false)) {
        return;
      }
      try {
        const suffix = appId === "" ? "" : `&app_id=${encodeURIComponent(appId)}`;
        const res = await fetch(`/api/settings?resource=${resource}${suffix}`);
        const body: unknown = await res.json();
        setLists((prev) => ({ ...prev, [resource]: body }));
      } catch {
        setLists((prev) => ({ ...prev, [resource]: { error: "load failed" } }));
      }
    },
    [appId],
  );

  useEffect(() => {
    for (const section of SECTIONS) {
      if (!section.appScoped || appId !== "") {
        void load(section.resource);
      }
    }
  }, [appId, load]);

  async function submit(resource: Resource) {
    const raw = draft[resource];
    let payload: Record<string, unknown>;
    try {
      payload = raw === undefined ? {} : (JSON.parse(raw) as Record<string, unknown>);
    } catch {
      return;
    }
    const suffix = appId === "" ? "" : `&app_id=${encodeURIComponent(appId)}`;
    await fetch(`/api/settings${suffix}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ resource, payload }),
    });
    void load(resource);
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <p className="text-sm text-neutral-400">
        app_id for scoped sections:{" "}
        <span data-testid="settings-app-id" className="font-mono">
          {appId === "" ? "(none — pass ?app_id=)" : appId}
        </span>
      </p>
      {SECTIONS.map((section) => (
        <section key={section.resource} className="space-y-2 rounded border border-neutral-700 p-4">
          <h2 className="font-medium">{section.label}</h2>
          <pre
            data-testid={`settings-${section.resource}`}
            className="max-h-48 overflow-auto rounded bg-neutral-900 p-3 text-xs"
          >
            {JSON.stringify(lists[section.resource] ?? null, null, 2)}
          </pre>
          <textarea
            rows={3}
            placeholder='{"key":"value"}'
            value={draft[section.resource] ?? ""}
            onChange={(e) => setDraft((prev) => ({ ...prev, [section.resource]: e.target.value }))}
            className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2 font-mono text-xs"
          />
          <button
            type="button"
            onClick={() => void submit(section.resource)}
            className="rounded bg-emerald-500 px-3 py-1 font-medium text-black hover:bg-emerald-400"
          >
            Save {section.label}
          </button>
        </section>
      ))}
    </div>
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={null}>
      <SettingsInner />
    </Suspense>
  );
}
