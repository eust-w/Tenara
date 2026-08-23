import { z } from "zod";
import type { APIRequestContext } from "@playwright/test";

const idempotencyKeySchema = z.string().uuid();

export interface ApiClientOptions {
  readonly bearer?: string;
  readonly baseUrl: string;
}

export class ApiClient {
  constructor(
    private readonly request: APIRequestContext,
    private readonly options: ApiClientOptions,
  ) {}

  async get(path: string): Promise<Response> {
    return this.request.get(this.url(path), { headers: this.headers() });
  }

  async post(path: string, body: unknown): Promise<Response> {
    return this.request.post(this.url(path), {
      headers: this.headers(),
      data: body,
    });
  }

  async put(path: string, body: unknown): Promise<Response> {
    return this.request.put(this.url(path), {
      headers: this.headers(),
      data: body,
    });
  }

  async delete(path: string): Promise<Response> {
    return this.request.delete(this.url(path), { headers: this.headers() });
  }

  private url(path: string): string {
    return `${this.options.baseUrl}${path}`;
  }

  private headers(): Record<string, string> {
    const key = crypto.randomUUID();
    idempotencyKeySchema.parse(key);
    const headers: Record<string, string> = {
      "Idempotency-Key": key,
    };
    if (this.options.bearer !== undefined) {
      headers.Authorization = `Bearer ${this.options.bearer}`;
    }
    return headers;
  }
}
