"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

export default function LoginPage() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    const form = new FormData(event.currentTarget);
    const res = await fetch("/api/session", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        email: String(form.get("email") ?? ""),
        password: String(form.get("password") ?? ""),
      }),
    });
    if (res.ok) {
      router.push("/");
      return;
    }
    setError("invalid email or password");
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Sign in</h1>
      <form onSubmit={onSubmit} className="space-y-4">
        <input
          name="email"
          type="email"
          required
          placeholder="email"
          className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
        />
        <input
          name="password"
          type="password"
          required
          placeholder="password"
          className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
        />
        {error !== null && <p className="text-sm text-red-400">{error}</p>}
        <button
          type="submit"
          className="w-full rounded bg-emerald-500 px-3 py-2 font-medium text-black hover:bg-emerald-400"
        >
          Sign in
        </button>
      </form>
      <p className="text-sm text-neutral-400">
        No account?{" "}
        <Link className="underline" href="/register">
          Register
        </Link>{" "}
        ·{" "}
        <Link className="underline" href="/forgot">
          Forgot password
        </Link>
      </p>
    </div>
  );
}
