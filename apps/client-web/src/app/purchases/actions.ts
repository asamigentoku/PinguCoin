"use server";

import { revalidatePath } from "next/cache";
import { sellerGraphql } from "@/lib/seller";
import { createOrder, friendlyError } from "@/lib/shop";

export type PurchaseResult = { purchased: number[]; error?: string };

// カートの商品を1つずつ注文する。途中で失敗したら、そこまでに購入できた商品のIDと、
// 失敗の理由を返す(購入済みの商品だけをカートから外せるようにするため)。
export async function purchase(items: { productId: number; quantity: number }[], attemptId: string): Promise<PurchaseResult> {
  const purchased: number[] = [];
  for (const item of items) {
    if (!Number.isInteger(item.productId) || !Number.isInteger(item.quantity) || item.quantity < 1 || item.quantity > 99) {
      return { purchased, error: "カートの内容が正しくありません。" };
    }
    try {
      // 同じattemptIdでの再送は、pingu-api側で同じ注文として扱われる。
      const order = await createOrder(item.productId, item.quantity, `${attemptId}:${item.productId}`);
      if (order.status !== "paid") return { purchased, error: "決済が完了しませんでした。" };
      purchased.push(item.productId);
    } catch (error) {
      return { purchased, error: friendlyError(error) };
    }
  }
  revalidatePath("/purchases");
  revalidatePath("/points");
  return { purchased };
}

// 商品ファイル1つの、有効期限つきダウンロードURLを発行する(出品者本人と購入者だけが取得できる)。
export async function getDownloadUrl(assetId: number): Promise<{ ok: true; url: string } | { ok: false; error: string }> {
  try {
    const data = await sellerGraphql<{ getProductAssetDownloadUrl: { downloadUrl: string } }>(
      `mutation Download($id: Int!) { getProductAssetDownloadUrl(assetId: $id) { downloadUrl } }`,
      { id: assetId },
    );
    return { ok: true, url: data.getProductAssetDownloadUrl.downloadUrl };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : "ダウンロードURLを発行できませんでした。" };
  }
}
