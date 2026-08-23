"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

interface RepoItem {
  fullName: string;
  privateRepo: boolean;
  defaultBranch: string;
}

export default function GithubPage() {
  const router = useRouter();
  const [repos, setRepos] = useState<RepoItem[]>([]);
  const [query, setQuery] = useState("");
  const [picked, setPicked] = useState<string | null>(null);
  const [branch, setBranch] = useState("");
  const [env, setEnv] = useState("prod");

  useEffect(() => {
    let cancelled = false;
    fetch(`/api/github/repos?q=${encodeURIComponent(query)}`)
      .then((res) => (res.ok ? res.json() : null))
      .then((body: unknown) => {
        if (cancelled || body === null) return;
        setRepos((body as { repos?: RepoItem[] }).repos ?? []);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [query]);

  async function createApp() {
    if (picked === null) return;
    const res = await fetch("/api/apps", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        repo: picked,
        branch,
        env,
        name: picked.split("/")[1] ?? picked,
      }),
    });
    if (!res.ok) return;
    const body = (await res.json()) as { id?: string };
    router.push(`/apps/${body.id ?? ""}`);
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Connect GitHub</h1>
      <a
        href="/api/github/start"
        className="inline-block rounded bg-emerald-500 px-3 py-2 font-medium text-black hover:bg-emerald-400"
      >
        Bind GitHub account
      </a>

      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="search repositories"
        className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
      />
      <ul className="space-y-1">
        {repos.map((repo) => (
          <li key={repo.fullName}>
            <button
              type="button"
              onClick={() => {
                setPicked(repo.fullName);
                setBranch(repo.defaultBranch);
              }}
              className={`w-full rounded border px-3 py-2 text-left ${
                picked === repo.fullName ? "border-emerald-500" : "border-neutral-700"
              }`}
            >
              {repo.fullName}
              {repo.privateRepo ? " (private)" : ""} · default: {repo.defaultBranch}
            </button>
          </li>
        ))}
      </ul>

      {picked !== null && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void createApp();
          }}
          className="space-y-3"
        >
          <input
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            placeholder="branch"
            className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
          />
          <select
            value={env}
            onChange={(e) => setEnv(e.target.value)}
            className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
          >
            <option value="prod">prod</option>
            <option value="staging">staging</option>
          </select>
          <button
            type="submit"
            className="w-full rounded bg-emerald-500 px-3 py-2 font-medium text-black hover:bg-emerald-400"
          >
            Create app
          </button>
        </form>
      )}
    </div>
  );
}
