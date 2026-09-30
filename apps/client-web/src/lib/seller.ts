import { auth } from "@clerk/nextjs/server";
import { graphql } from "./api";
import { PRODUCT_FILE_PURPOSE_ID } from "./listing";
import type { Product } from "./types";

const fields = `id userId categoryId name description imageUrl fileUrl price status createdAt updatedAt`;

// ログイン中ユーザーのClerkセッショントークンを付けてpingu-apiを呼ぶ。
export async function sellerGraphql<T>(query: string, variables?: Record<string, unknown>) {
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) throw new Error("ログインが必要です。");
  return graphql<T>(query, variables, token);
}

export async function getMyProducts(): Promise<Product[]> {
  const me = await sellerGraphql<{ me: { id: number } | null }>(`query SellerMe { me { id } }`);
  if (!me.me) return [];
  const data = await sellerGraphql<{ products: Product[] }>(`query SellerProducts($userId: Int!) { products(userId: $userId) { ${fields} } }`, { userId: me.me.id });
  return data.products.sort((a, b) => b.id - a.id);
}

export async function getMyProduct(id: number): Promise<Product | null> {
  return (await getMyProducts()).find((product) => product.id === id) ?? null;
}

export type ProductFile = { id: number; productId: number; originalFilename: string; contentType: string; fileSize: number; sortOrder: number };

// 商品の販売ファイル一覧。pingu-api側で、出品者本人と購入済みのユーザー以外には返らない。
export async function getProductFiles(productId: number): Promise<ProductFile[]> {
  const data = await sellerGraphql<{ productAssets: ProductFile[] }>(
    `query ProductFiles($id: Int!, $purpose: Int!) { productAssets(productId: $id, purposeId: $purpose) { id productId originalFilename contentType fileSize sortOrder } }`,
    { id: productId, purpose: PRODUCT_FILE_PURPOSE_ID },
  );
  return data.productAssets.sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
}
