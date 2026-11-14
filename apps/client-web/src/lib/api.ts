import { HttpStatusError, withRetry } from "./retry";
import type { Product } from "./types";

export const bffBase = (process.env.BFF_API_URL ?? "http://localhost:8082/api/v1").replace(/\/$/, "");
export type RestOptions = { retry?: boolean };
export type ExchangeRatePayload = { base: "JPY"; rates: Record<string, number>; updatedAt: string };
export type ExchangeRateManifest = { version: string; file: string; updatedAt: string };

export async function bffFetch<T>(path: string, init: RequestInit = {}, token?: string, options: RestOptions = {}): Promise<T> {
  const send = async () => {
    const response = await fetch(`${bffBase}${path}`, {
      ...init,
      headers: { ...(init.body ? { "content-type": "application/json" } : {}), ...(token ? { authorization: `Bearer ${token}` } : {}), ...init.headers },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) {
      const payload = (await response.json().catch(() => null)) as { message?: string } | null;
      throw new HttpStatusError(response.status, payload?.message ?? `BFF returned ${response.status}`);
    }
    if (response.status === 204) return undefined as T;
    return response.json() as Promise<T>;
  };
  return options.retry ? withRetry(send) : send();
}

export async function getProducts(): Promise<Product[]> {
  const products = await bffFetch<Product[]>("/products", {}, undefined, { retry: true });
  return products.filter((product) => product.status !== "draft");
}

export async function getProduct(id: number): Promise<Product | null> {
  try { return await bffFetch<Product>(`/products/${id}`, {}, undefined, { retry: true }); }
  catch (error) { if (error instanceof HttpStatusError && error.status === 404) return null; throw error; }
}

export async function getExchangeRates(): Promise<ExchangeRatePayload> {
  const manifestResponse = await fetch(`${bffBase}/exchange-rates/latest.json`, {
    next: { revalidate: 60 },
    signal: AbortSignal.timeout(5000),
  });
  if (!manifestResponse.ok) throw new HttpStatusError(manifestResponse.status, "Exchange-rate manifest unavailable");
  const manifest = await manifestResponse.json() as ExchangeRateManifest;
  if (!/^\d{14}\.json$/.test(manifest.file)) throw new Error("Invalid exchange-rate snapshot filename");

  const snapshotResponse = await fetch(`${bffBase}/exchange-rates/${manifest.file}`, {
    cache: "force-cache",
    signal: AbortSignal.timeout(5000),
  });
  if (!snapshotResponse.ok) throw new HttpStatusError(snapshotResponse.status, "Exchange-rate snapshot unavailable");
  return snapshotResponse.json() as Promise<ExchangeRatePayload>;
}
