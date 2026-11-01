import { auth } from "@clerk/nextjs/server";
import { bffFetch, type RestOptions } from "./api";
import { PRODUCT_FILE_PURPOSE_ID } from "./listing";
import type { Product } from "./types";

export async function sellerFetch<T>(path: string, init: RequestInit = {}, options?: RestOptions): Promise<T> {
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) throw new Error("ログインが必要です。");
  return bffFetch<T>(path, init, token, options);
}

export async function getMyProducts(): Promise<Product[]> {
  const products = await sellerFetch<Product[]>("/me/products", {}, { retry: true });
  return products.sort((a, b) => b.id - a.id);
}
export async function getMyProduct(id: number): Promise<Product | null> { return (await getMyProducts()).find((p) => p.id === id) ?? null; }
export type ProductFile = { id: number; productId: number; originalFilename: string; contentType: string; fileSize: number; sortOrder: number };
export async function getProductFiles(productId: number): Promise<ProductFile[]> {
  const files = await sellerFetch<ProductFile[]>(`/products/${productId}/assets?purposeId=${PRODUCT_FILE_PURPOSE_ID}`, {}, { retry: true });
  return files.sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
}
