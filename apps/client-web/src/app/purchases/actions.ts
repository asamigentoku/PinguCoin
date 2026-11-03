"use server";

import { revalidatePath } from "next/cache";
import { sellerFetch } from "@/lib/seller";
import { createOrder, friendlyError, waitForOrder } from "@/lib/shop";
import type { PaymentMethod } from "@/lib/shop";

export type PurchaseResult = { purchased: number[]; error?: string; actionUrl?: string };
export async function purchase(items: { productId: number; quantity: number }[], paymentMethod: PaymentMethod, attemptId: string): Promise<PurchaseResult> {
  const purchased: number[] = [];
  for (const item of items) {
    if (!Number.isInteger(item.productId) || !Number.isInteger(item.quantity) || item.quantity < 1 || item.quantity > 99) return { purchased, error: "カートの内容が正しくありません。" };
    try { const order = await waitForOrder(await createOrder(item.productId, item.quantity, paymentMethod, `${attemptId}:${item.productId}`)); if (order.status === "payment_action_required" && order.action_url) { purchased.push(item.productId); return { purchased, actionUrl: order.action_url }; } if (order.status !== "paid") return { purchased, error: "決済が完了しませんでした。" }; purchased.push(item.productId); }
    catch (error) { return { purchased, error: friendlyError(error) }; }
  }
  revalidatePath("/purchases"); revalidatePath("/points"); return { purchased };
}
export async function getDownloadUrl(assetId: number): Promise<{ ok: true; url: string } | { ok: false; error: string }> {
  try { const data = await sellerFetch<{ downloadUrl: string }>(`/assets/${assetId}/download`); return { ok: true, url: data.downloadUrl }; }
  catch (error) { return { ok: false, error: error instanceof Error ? error.message : "ダウンロードURLを発行できませんでした。" }; }
}
