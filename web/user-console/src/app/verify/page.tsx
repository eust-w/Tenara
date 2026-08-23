type SearchParams = Promise<{ token?: string }>;

async function verifyToken(token: string): Promise<{ ok: boolean }> {
  const api = process.env.TENARA_API_URL ?? "http://127.0.0.1:8080";
  try {
    const res = await fetch(`${api}/v1/auth/verify?token=${encodeURIComponent(token)}`, {
      cache: "no-store",
    });
    return { ok: res.ok };
  } catch {
    return { ok: false };
  }
}

export default async function VerifyPage({ searchParams }: { searchParams: SearchParams }) {
  const { token } = await searchParams;
  if (token === undefined) {
    return <p className="text-red-400">missing verification token</p>;
  }
  const { ok } = await verifyToken(token);
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">{ok ? "Email verified" : "Verification failed"}</h1>
      <p className="text-neutral-400">
        {ok ? "You can now sign in with your credentials." : "The link may have expired."}
      </p>
    </div>
  );
}
