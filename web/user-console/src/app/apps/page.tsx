"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

interface AppCard {
  id: string;
  name: string;
  env: string;
  state: string;
}

export default function AppsPage() {
  const [apps, setApps] = useState<AppCard[]>([]);

  useEffect(() => {
    fetch("/api/apps")
      .then((res) => (res.ok ? res.json() : null))
      .then((body: unknown) => {
        setApps((body as { apps?: AppCard[] })?.apps ?? []);
      })
      .catch(() => {});
  }, []);

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Your apps</h1>
      <ul className="space-y-2">
        {apps.map((a) => (
          <li key={a.id}>
            <Link
              href={`/apps/${a.id}`}
              className="block rounded border border-neutral-700 px-3 py-2 hover:border-emerald-500"
            >
              <span className="font-medium">{a.name}</span> · {a.env} ·{" "}
              <span className="text-neutral-400">{a.state}</span>
            </Link>
          </li>
        ))}
      </ul>
      {apps.length === 0 && (
        <p className="text-neutral-400">No apps yet — connect GitHub to create one.</p>
      )}
    </div>
  );
}
