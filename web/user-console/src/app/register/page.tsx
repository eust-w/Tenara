"use client";

import Link from "next/link";
import { useState } from "react";

import { passwordIssue } from "@/lib/password-policy";

export default function RegisterPage() {
  const [issue, setIssue] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");
    const policyError = passwordIssue(password);
    if (policyError !== null) {
      setIssue(policyError);
      return;
    }
    setIssue(null);
    const res = await fetch("/api/register", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        email: String(form.get("email") ?? ""),
        password,
      }),
    });
    if (res.ok) {
      setSubmitted(true);
      return;
    }
    setIssue("registration failed");
  }

  if (submitted) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-semibold">Check your inbox</h1>
        <p className="text-neutral-400">
          We sent a verification link to your email address. Follow it to activate the account.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Create account</h1>
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
          placeholder="password (min 8, letters+numbers)"
          className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
        />
        {issue !== null && <p className="text-sm text-red-400">{issue}</p>}
        <button
          type="submit"
          className="w-full rounded bg-emerald-500 px-3 py-2 font-medium text-black hover:bg-emerald-400"
        >
          Register
        </button>
      </form>
      <p className="text-sm text-neutral-400">
        Already registered?{" "}
        <Link className="underline" href="/login">
          Sign in
        </Link>
      </p>
    </div>
  );
}
