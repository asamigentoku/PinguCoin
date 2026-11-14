import { bffBase } from "@/lib/api";

export async function GET() {
  const response = await fetch(`${bffBase}/exchange-rates`, { cache: "no-store", signal: AbortSignal.timeout(5000) });
  return new Response(await response.text(), {
    status: response.status,
    headers: { "content-type": response.headers.get("content-type") ?? "application/json" },
  });
}
