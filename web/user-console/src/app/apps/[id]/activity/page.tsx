"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import {
  buildTimeline,
  logsQuery,
  rollbackConfirmText,
  type LogsSource,
  type RevisionRow,
} from "@/lib/timeline";

export default function ActivityPage() {
  const params = useParams<{ id: string }>();

  const [rows, setRows] = useState<RevisionRow[]>([]);
  const [source, setSource] = useState<LogsSource>("app");
  const [tail, setTail] = useState(200);
  const [logs, setLogs] = useState<string | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<RevisionRow | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    fetch(`/api/apps/${params.id}/revisions`)
      .then((res) => (res.ok ? res.json() : null))
      .then((body: unknown) => {
        setRows(
          buildTimeline((body as { revisions?: Array<Record<string, unknown>> })?.revisions ?? []),
        );
      })
      .catch(() => {});
  }, [params.id]);

  async function loadLogs() {
    const res = await fetch(`/api/apps/${params.id}/logs${logsQuery(source, tail)}`);
    if (!res.ok) {
      setLogs(`logs unavailable (${res.status})`);
      return;
    }
    setLogs(await res.text());
  }

  async function doRollback() {
    if (confirmTarget === null) return;
    const res = await fetch(`/api/apps/${params.id}/rollback`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ target_revision: confirmTarget.number }),
    });
    setMessage(
      res.ok
        ? `rolling back to revision ${confirmTarget.number}`
        : `rollback failed (${res.status})`,
    );
    setConfirmTarget(null);
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Activity</h1>

      <section className="space-y-2">
        <h2 className="font-medium">Deployment timeline</h2>
        <ul className="space-y-1 text-sm">
          {rows.map((row) => (
            <li key={row.number} className="rounded border border-neutral-700 px-3 py-2">
              revision {row.number} · image {row.digest.slice(0, 14)} ·{" "}
              <span className="text-neutral-400">{row.state}</span>
              <button
                type="button"
                onClick={() => setConfirmTarget(row)}
                className="ml-3 rounded border border-neutral-600 px-2 py-0.5 hover:border-emerald-500"
              >
                Roll back here
              </button>
            </li>
          ))}
        </ul>
        {rows.length === 0 && <p className="text-neutral-500">no revisions yet</p>}
      </section>

      {confirmTarget !== null && (
        <section
          role="dialog"
          aria-label="confirm rollback"
          className="space-y-2 rounded border border-amber-500 p-4"
        >
          <p>{rollbackConfirmText(confirmTarget)}</p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => void doRollback()}
              className="rounded bg-emerald-500 px-3 py-1 font-medium text-black"
            >
              Confirm rollback
            </button>
            <button
              type="button"
              onClick={() => setConfirmTarget(null)}
              className="rounded border px-3 py-1"
            >
              Cancel
            </button>
          </div>
        </section>
      )}

      <section className="space-y-2">
        <h2 className="font-medium">Logs</h2>
        <div className="flex items-center gap-3">
          {(["build", "app"] as const).map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setSource(s)}
              className={source === s ? "font-medium underline" : "text-neutral-400"}
            >
              {s}
            </button>
          ))}
          <input
            type="number"
            min={1}
            value={tail}
            onChange={(e) => setTail(Number(e.target.value) || 1)}
            className="w-24 rounded border border-neutral-700 bg-neutral-900 px-2 py-1"
          />
          <button
            type="button"
            onClick={() => void loadLogs()}
            className="rounded border px-2 py-1"
          >
            Load logs
          </button>
        </div>
        {logs !== null && (
          <pre className="max-h-80 overflow-auto rounded bg-neutral-900 p-3 text-xs">{logs}</pre>
        )}
      </section>

      {message !== null && <p className="text-sm text-emerald-400">{message}</p>}
    </div>
  );
}
