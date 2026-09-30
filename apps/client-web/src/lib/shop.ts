import { auth } from "@clerk/nextjs/server";

// pingu-apiのRESTエンドポイント(注文・ポイント)。GraphQLのURLから組み立てる。
const base = (process.env.PINGU_API_URL ?? "http://localhost:8082/api/v1/graphql").replace(/\/graphql$/, "");

export type PointTransaction = { id: number; amount: number; type: string; reason: string; balance_after: number; created_at: string };
export type PointSummary = { balance: number; transactions: PointTransaction[] };
export type Order = { id: number; user_id: number; product_id: number; quantity: number; unit_price: number; total_amount: number; payment_id: number; status: string };

// エラー時はpingu-apiが返す {reason, message} のmessageを、そのままErrorにする。
async function shopFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) throw new Error("ログインが必要です。");
  const response = await fetch(`${base}${path}`, {
    ...init,
    headers: { ...init?.headers, authorization: `Bearer ${token}`, "content-type": "application/json" },
    cache: "no-store",
    signal: AbortSignal.timeout(10000),
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { message?: string } | null;
    throw new Error(body?.message ?? `pingu-apiがエラーを返しました(${response.status})`);
  }
  return (await response.json()) as T;
}

export const getPoints = () => shopFetch<PointSummary>("/points");
export const getOrders = () => shopFetch<Order[]>("/orders");

// idempotencyKeyは1回の購入操作ごとに発行し、リトライや二重クリックでも二重に課金されないようにする。
export const createOrder = (productId: number, quantity: number, idempotencyKey: string) =>
  shopFetch<Order>("/orders", {
    method: "POST",
    headers: { "idempotency-key": idempotencyKey },
    body: JSON.stringify({ product_id: productId, quantity, payment_method: "point" }),
  });

const messages: Record<string, string> = {
  "insufficient points": "ポイントが足りません。",
  "insufficient stock": "在庫がありません。",
};
export const friendlyError = (error: unknown) => {
  const message = error instanceof Error ? error.message : "";
  return messages[message] ?? (message || "購入に失敗しました。");
};
