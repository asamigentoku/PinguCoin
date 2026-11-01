"use server";

import { revalidatePath } from "next/cache";
import { PRODUCT_FILE_PURPOSE_ID, validateListing, type ListingInput, type ListingStatus } from "@/lib/listing";
import { getMyProduct, getProductFiles, sellerFetch, type ProductFile } from "@/lib/seller";

export type ActionResult<T = null> = { ok: true; data: T } | { ok: false; error: string };
export type UploadTargets = { image: { blobEndpoint: string; container: string; mainImagePrefix: string; sasToken: string }; files: { blobEndpoint: string; container: string; pathPrefix: string; sasToken: string } };
async function run<T>(task: () => Promise<T>): Promise<ActionResult<T>> { try { return { ok: true, data: await task() }; } catch (error) { return { ok: false, error: error instanceof Error ? error.message : "処理に失敗しました。" }; } }
function refresh(id?: number) { revalidatePath("/sell/manage"); revalidatePath("/products"); revalidatePath("/purchases"); if (id) revalidatePath(`/products/${id}`); }
function body(input: ListingInput) { return JSON.stringify({ categoryId: input.categoryId, name: input.name.trim(), description: input.description, price: input.price, status: input.status }); }

export async function createListing(input: ListingInput): Promise<ActionResult<{ productId: number }>> {
  const invalid = validateListing(input); if (invalid) return { ok: false, error: invalid };
  return run(async () => { const product = await sellerFetch<{ id: number }>("/products", { method: "POST", body: body(input) }); refresh(); return { productId: product.id }; });
}
export async function updateListing(id: number, input: ListingInput): Promise<ActionResult> {
  const invalid = validateListing(input); if (invalid) return { ok: false, error: invalid };
  return run(async () => { await sellerFetch(`/products/${id}`, { method: "PATCH", body: body(input) }); refresh(id); return null; });
}
export async function setListingStatus(id: number, status: ListingStatus): Promise<ActionResult> {
  return run(async () => { const product = await getMyProduct(id); if (!product) throw new Error("商品が見つかりません。"); if (status === "available" && (await getProductFiles(id)).length === 0) throw new Error("公開するには商品ファイルをアップロードしてください。"); await sellerFetch(`/products/${id}`, { method: "PATCH", body: body({ name: product.name, description: product.description, categoryId: product.categoryId, price: product.price, status }) }); refresh(id); return null; });
}
export async function deleteListing(id: number): Promise<ActionResult> {
  return run(async () => { const product = await getMyProduct(id); if (!product) throw new Error("商品が見つかりません。"); if (product.imageUrl) await sellerFetch(`/products/${id}/image`, { method: "DELETE", body: JSON.stringify({ fileUrl: product.imageUrl }) }).catch(() => undefined); for (const file of await getProductFiles(id).catch(() => [] as ProductFile[])) await sellerFetch(`/assets/${file.id}`, { method: "DELETE" }).catch(() => undefined); await sellerFetch(`/products/${id}`, { method: "DELETE" }); refresh(id); return null; });
}
export async function prepareUploads(productId: number): Promise<ActionResult<UploadTargets>> { return run(() => sellerFetch<UploadTargets>(`/products/${productId}/upload-targets?purposeId=${PRODUCT_FILE_PURPOSE_ID}`)); }
export async function confirmMainImage(productId: number, fileUrl: string, previousUrl?: string): Promise<ActionResult> {
  return run(async () => { await sellerFetch(`/products/${productId}/image`, { method: "POST", body: JSON.stringify({ fileUrl }) }); if (previousUrl && previousUrl !== fileUrl) await sellerFetch(`/products/${productId}/image`, { method: "DELETE", body: JSON.stringify({ fileUrl: previousUrl }) }).catch(() => undefined); refresh(productId); return null; });
}
export type UploadedFileInfo = { originalFilename: string; contentType: string; fileSize: number; sortOrder: number };
export async function confirmProductFile(productId: number, fileUrl: string, info: UploadedFileInfo): Promise<ActionResult<ProductFile>> {
  return run(async () => { const file = await sellerFetch<ProductFile>(`/products/${productId}/assets`, { method: "POST", body: JSON.stringify({ purposeId: PRODUCT_FILE_PURPOSE_ID, fileUrl, ...info }) }); refresh(productId); return file; });
}
export async function removeProductFile(assetId: number): Promise<ActionResult> { return run(async () => { await sellerFetch(`/assets/${assetId}`, { method: "DELETE" }); refresh(); return null; }); }
