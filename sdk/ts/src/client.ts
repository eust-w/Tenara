// Hand-written platform client over the generated OpenAPI types (RB§8 §9).
// The bearer token lives only inside the instance — it is never persisted.

// Ambient: provided by every browser and Node runtime; absent from the
// ESNext-only compiler lib on purpose.
declare function setTimeout(callback: () => void, milliseconds: number): void;

export type FetchLike = (
  url: string,
  init?: {
    method?: string;
    headers?: Record<string, string>;
    body?: string;
  },
) => Promise<{ status: number; text(): Promise<string> }>;

const globalScope = globalThis as unknown as {
  fetch?: FetchLike;
  crypto?: { randomUUID(): string };
};

function defaultFetch(): FetchLike {
  if (typeof globalScope.fetch === "function") {
    return globalScope.fetch;
  }
  throw new Error("no fetch implementation available; pass fetchImpl");
}

function defaultKeyFactory(): string {
  const c = globalScope.crypto;
  if (c && typeof c.randomUUID === "function") {
    return c.randomUUID();
  }
  throw new Error("no crypto.randomUUID available; pass idempotencyKeyFactory");
}

const IDEMPOTENT_METHODS = new Set(["GET", "HEAD", "PUT", "DELETE"]);
const MUTATING_METHODS = new Set(["POST", "PATCH", "PUT", "DELETE"]);

export interface RequestResult<T = unknown> {
  status: number;
  data: T;
}

export interface TenaraClientOptions {
  baseURL: string;
  token: string;
  fetchImpl?: FetchLike;
  idempotencyKeyFactory?: () => string;
  maxRetries?: number;
  backoffMs?: number;
}

export class TenaraClient {
  private readonly tokenValue: string;
  private readonly baseURL: string;
  private readonly fetchImpl: FetchLike;
  private readonly keyFactory: () => string;
  private readonly maxRetries: number;
  private readonly backoffMs: number;

  constructor(options: TenaraClientOptions) {
    this.tokenValue = options.token;
    this.baseURL = options.baseURL.replace(/\/$/, "");
    this.fetchImpl = options.fetchImpl ?? defaultFetch();
    this.keyFactory = options.idempotencyKeyFactory ?? defaultKeyFactory;
    this.maxRetries = options.maxRetries ?? 3;
    this.backoffMs = options.backoffMs ?? 100;
  }

  async request<T>(
    method: string,
    path: string,
    body?: unknown,
    explicitKey?: string,
  ): Promise<RequestResult<T>> {
    const verb = method.toUpperCase();
    const needsKey = MUTATING_METHODS.has(verb);
    const key = explicitKey ?? (needsKey ? this.keyFactory() : undefined);

    let attempt = 0;
    for (;;) {
      const headers: Record<string, string> = {
        Authorization: `Bearer ${this.tokenValue}`,
      };
      if (key !== undefined) {
        headers["Idempotency-Key"] = key;
      }
      if (body !== undefined) {
        headers["Content-Type"] = "application/json";
      }

      const init: { method: string; headers: Record<string, string>; body?: string } = {
        method: verb,
        headers,
      };
      if (body !== undefined) {
        init.body = JSON.stringify(body);
      }

      const resp = await this.fetchImpl(this.baseURL + path, init);

      const retryable = (resp.status === 429 || resp.status >= 500) && IDEMPOTENT_METHODS.has(verb);

      if (!retryable || attempt >= this.maxRetries) {
        const text = await resp.text();
        let data: unknown = text;
        try {
          data = JSON.parse(text) as unknown;
        } catch {
          // keep raw body text as data
        }
        return { status: resp.status, data: data as T };
      }

      attempt += 1;
      await new Promise<void>((resolve) => {
        setTimeout(resolve, this.backoffMs * 2 ** (attempt - 1));
      });
    }
  }

  get<T>(path: string): Promise<RequestResult<T>> {
    return this.request<T>("GET", path);
  }

  post<T>(path: string, body?: unknown, explicitKey?: string): Promise<RequestResult<T>> {
    return this.request<T>("POST", path, body, explicitKey);
  }

  put<T>(path: string, body?: unknown, explicitKey?: string): Promise<RequestResult<T>> {
    return this.request<T>("PUT", path, body, explicitKey);
  }

  del<T>(path: string, explicitKey?: string): Promise<RequestResult<T>> {
    return this.request<T>("DELETE", path, undefined, explicitKey);
  }
}
