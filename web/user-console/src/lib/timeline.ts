// Deployment timeline + logs query helpers (RB§26 §33).

export interface RevisionRow {
  number: number;
  shaShort: string;
  digest: string;
  state: string;
}

export type LogsSource = "build" | "app";

export function buildTimeline(revisions: Array<Record<string, unknown>>): RevisionRow[] {
  const rows: RevisionRow[] = [];
  for (const raw of revisions) {
    const number = typeof raw.number === "number" ? raw.number : NaN;
    const digest = typeof raw.digest === "string" ? raw.digest : "";
    const state = typeof raw.state === "string" ? raw.state : "";
    if (Number.isNaN(number) || digest === "") continue;
    rows.push({ number, shaShort: digest.slice(7, 14), digest, state });
  }
  return rows.sort((a, b) => b.number - a.number);
}

export function rollbackConfirmText(target: RevisionRow): string {
  return `Roll back to revision ${target.number} (image ${target.shaShort})? Current workload will be replaced.`;
}

export function logsQuery(source: LogsSource, tail: number): string {
  return `?source=${encodeURIComponent(source)}&tail=${Math.max(1, Math.floor(tail))}`;
}
