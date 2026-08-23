"use client";

import { useState } from "react";

export default function ForgotPage() {
  const [sent, setSent] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    await fetch("/api/forgot", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ email: String(form.get("email") ?? "") }),
    });
    setSent(true);
  }

  if (sent) {
    return (
      <p className="text-neutral-300">
        If the address exists, a password-reset link is on its way.
      </p>
    );
  }
  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <h1 className="text-2xl font-semibold">Reset password</h1>
      <input
        name="email"
        type="email"
        required
        placeholder="email"
        className="w-full rounded border border-neutral-700 bg-neutral-900 px-3 py-2"
      />
      <button
        type="submit"
        className="w-full rounded bg-emerald-500 px-3 py-2 font-medium text-black hover:bg-emerald-400"
      >
        Send reset link
      </button>
    </form>
  );
}
