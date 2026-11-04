import { auth } from "@clerk/nextjs/server";
import { HttpStatusError, withRetry } from "./retry";

const base = (process.env.BFF_API_URL ?? "http://localhost:8082/api/v1").replace(/\/$/, "");

export type PointTransaction = { id: number; amount: number; type: string; reason: string; balance_after: number; created_at: string; chain_sequence: number; previous_hash: string; transaction_hash: string };
export type PointSummary = { balance: number; transactions: PointTransaction[] };
export type ExternalPaymentMethod = "card" | "bank_transfer";
export type PaymentMethod = "point" | ExternalPaymentMethod;
export type Order = { id: number; user_id: number; product_id: number; quantity: number; unit_price: number; total_amount: number; payment_id: number; status: string; payment_method: PaymentMethod; currency: "POINT" | "JPY"; action_url?: string };
export type Payment = { id: number; amount: number; currency: string; status: string; purpose: string; action_url?: string };

// エラー時はbffが返す {reason, message} のmessageを、そのままErrorにする。
// 一時的な失敗(つながらない、502/503)のときは、少し待って再試行する(retry: true の呼び出しだけ)。
// 再試行してよいのは、同じ呼び出しを2回やっても結果が変わらないものだけ(下の3つ)。
async function shopFetch<T>(path: string, init?: RequestInit, retry = false): Promise<T> {
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) throw new Error("ログインが必要です。");
  const send = async () => {
    const response = await fetch(`${base}${path}`, {
      ...init,
      headers: { ...init?.headers, authorization: `Bearer ${token}`, "content-type": "application/json" },
      cache: "no-store",
      signal: AbortSignal.timeout(10000),
    });
    if (!response.ok) {
      const body = (await response.json().catch(() => null)) as { message?: string } | null;
      throw new HttpStatusError(response.status, body?.message ?? `bffがエラーを返しました(${response.status})`);
    }
    return (await response.json()) as T;
  };
  return retry ? withRetry(send) : send();
}

// 読み取り(GET)は、再試行してよい。
export const getPoints = () => shopFetch<PointSummary>("/points", undefined, true);
export const getOrders = () => shopFetch<Order[]>("/orders", undefined, true);
export const getOrder = (id: number) => shopFetch<Order>(`/orders/${id}`, undefined, true);

// idempotencyKeyは1回の購入操作ごとに発行する。同じキーの再送は、在庫・決済・注文を新たに作らず、最初の結果を返す
// (bff / orcan-api / payment-api のすべてで)。そのため、一時的な失敗のときの再試行(同じキーで送り直す)は、
// 二重に課金されない。キーが付いていない書き込みは、再試行してはいけない。
export const createOrder = (productId: number, quantity: number, paymentMethod: PaymentMethod, idempotencyKey: string) =>
  shopFetch<Order>(
    "/orders",
    {
      method: "POST",
      headers: { "idempotency-key": idempotencyKey },
      body: JSON.stringify({ product_id: productId, quantity, payment_method: paymentMethod, currency: paymentMethod === "point" ? "POINT" : "JPY" }),
    },
    true,
  );

export async function waitForOrder(order: Order): Promise<Order> {
  let current = order;
  for (let attempt = 0; attempt < 40; attempt++) {
    if (["paid", "failed", "rejected", "compensation_required", "payment_action_required"].includes(current.status)) return current;
    await new Promise((resolve) => setTimeout(resolve, 500));
    current = await getOrder(order.id);
  }
  throw new Error("注文処理がタイムアウトしました。購入履歴から状態を確認してください。");
}

export const createPointTopUp = (amount: number, paymentMethod: ExternalPaymentMethod, idempotencyKey: string) =>
  shopFetch<Payment>("/points/topups", { method: "POST", headers: { "idempotency-key": idempotencyKey }, body: JSON.stringify({ amount, payment_method: paymentMethod }) }, true);

const messages: Record<string, string> = {
  "insufficient points": "ポイントが足りません。",
  "insufficient stock": "在庫がありません。",
};
export const friendlyError = (error: unknown) => {
  const message = error instanceof Error ? error.message : "";
  return messages[message] ?? (message || "購入に失敗しました。");
};
