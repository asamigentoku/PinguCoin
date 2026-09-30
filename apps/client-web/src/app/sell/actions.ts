"use server";

import { revalidatePath } from "next/cache";
import { PRODUCT_FILE_PURPOSE_ID, validateListing, type ListingInput, type ListingStatus } from "@/lib/listing";
import { getMyProduct, getProductFiles, sellerGraphql, type ProductFile } from "@/lib/seller";

export type ActionResult<T = null> = { ok: true; data: T } | { ok: false; error: string };

export type UploadTargets = {
  image: { blobEndpoint: string; container: string; mainImagePrefix: string; sasToken: string };
  files: { blobEndpoint: string; container: string; pathPrefix: string; sasToken: string };
};

async function run<T>(task: () => Promise<T>): Promise<ActionResult<T>> {
  try {
    return { ok: true, data: await task() };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : "処理に失敗しました。" };
  }
}

function refresh(productId?: number) {
  revalidatePath("/sell/manage");
  revalidatePath("/products");
  revalidatePath("/purchases");
  if (productId) revalidatePath(`/products/${productId}`);
}

// userId は pingu-api 側でログイン中のユーザーに固定される(ここで渡す値は使われない)。
function toInput(input: ListingInput) {
  return { userId: 0, categoryId: input.categoryId, name: input.name.trim(), description: input.description, price: input.price, status: input.status };
}

export async function createListing(input: ListingInput): Promise<ActionResult<{ productId: number }>> {
  const invalid = validateListing(input);
  if (invalid) return { ok: false, error: invalid };
  return run(async () => {
    const data = await sellerGraphql<{ createProduct: { id: number } }>(
      `mutation CreateListing($input: CreateProductInput!) { createProduct(input: $input) { id } }`,
      { input: toInput(input) },
    );
    refresh();
    return { productId: data.createProduct.id };
  });
}

export async function updateListing(id: number, input: ListingInput): Promise<ActionResult> {
  const invalid = validateListing(input);
  if (invalid) return { ok: false, error: invalid };
  return run(async () => {
    await sellerGraphql(`mutation UpdateListing($id: Int!, $input: UpdateProductInput!) { updateProduct(id: $id, input: $input) { id } }`, { id, input: toInput(input) });
    refresh(id);
    return null;
  });
}

export async function setListingStatus(id: number, status: ListingStatus): Promise<ActionResult> {
  return run(async () => {
    const product = await getMyProduct(id);
    if (!product) throw new Error("商品が見つかりません。");
    if (status === "available" && (await getProductFiles(id)).length === 0) throw new Error("公開するには、商品ファイルを1つ以上アップロードしてください。");
    await sellerGraphql(`mutation SetListingStatus($id: Int!, $input: UpdateProductInput!) { updateProduct(id: $id, input: $input) { id } }`, {
      id,
      input: toInput({ name: product.name, description: product.description, categoryId: product.categoryId, price: product.price, status }),
    });
    refresh(id);
    return null;
  });
}

export async function deleteListing(id: number): Promise<ActionResult> {
  return run(async () => {
    const product = await getMyProduct(id);
    if (!product) throw new Error("商品が見つかりません。");
    // Blob上の画像とファイルも消してから商品を削除する(Blobの削除に失敗しても商品の削除は続ける)。
    if (product.imageUrl) {
      await sellerGraphql(`mutation($id: Int!, $url: String!) { deleteProductImageUpload(productId: $id, fileUrl: $url) { id } }`, { id, url: product.imageUrl }).catch(() => undefined);
    }
    for (const file of await getProductFiles(id).catch(() => [] as ProductFile[])) {
      await sellerGraphql(`mutation($id: Int!) { deleteProductAsset(assetId: $id) }`, { id: file.id }).catch(() => undefined);
    }
    await sellerGraphql(`mutation DeleteListing($id: Int!) { deleteProduct(id: $id) }`, { id });
    refresh(id);
    return null;
  });
}

// ブラウザからAzure Blobへ直接PUTするための署名付きURL(SAS)を、画像用とファイル用でまとめて取得する。
export async function prepareUploads(productId: number): Promise<ActionResult<UploadTargets>> {
  return run(async () => {
    const data = await sellerGraphql<{ getProductImageUploadUrl: UploadTargets["image"]; getProductAssetUploadUrl: UploadTargets["files"] }>(
      `mutation PrepareUploads($id: Int!, $purpose: Int!) {
        getProductImageUploadUrl(productId: $id) { blobEndpoint container mainImagePrefix sasToken }
        getProductAssetUploadUrl(productId: $id, purposeId: $purpose) { blobEndpoint container pathPrefix sasToken }
      }`,
      { id: productId, purpose: PRODUCT_FILE_PURPOSE_ID },
    );
    return { image: data.getProductImageUploadUrl, files: data.getProductAssetUploadUrl };
  });
}

export async function confirmMainImage(productId: number, fileUrl: string, previousUrl?: string): Promise<ActionResult> {
  return run(async () => {
    await sellerGraphql(`mutation ConfirmImage($id: Int!, $url: String!) { confirmProductImageUpload(productId: $id, fileUrl: $url) { product { id } } }`, { id: productId, url: fileUrl });
    // 差し替え前の画像は不要なので消す(失敗しても差し替え自体は成功扱い)。
    if (previousUrl && previousUrl !== fileUrl) {
      await sellerGraphql(`mutation($id: Int!, $url: String!) { deleteProductImageUpload(productId: $id, fileUrl: $url) { id } }`, { id: productId, url: previousUrl }).catch(() => undefined);
    }
    refresh(productId);
    return null;
  });
}

export type UploadedFileInfo = { originalFilename: string; contentType: string; fileSize: number; sortOrder: number };

// アップロード済みのファイルを、商品の販売ファイルとして登録する。何個でも登録できる。
export async function confirmProductFile(productId: number, fileUrl: string, info: UploadedFileInfo): Promise<ActionResult<ProductFile>> {
  return run(async () => {
    const data = await sellerGraphql<{ confirmProductAssetUpload: ProductFile }>(
      `mutation ConfirmProductFile($id: Int!, $purpose: Int!, $url: String!, $input: ConfirmProductAssetInput) {
        confirmProductAssetUpload(productId: $id, purposeId: $purpose, fileUrl: $url, input: $input) { id productId originalFilename contentType fileSize sortOrder }
      }`,
      { id: productId, purpose: PRODUCT_FILE_PURPOSE_ID, url: fileUrl, input: info },
    );
    refresh(productId);
    return data.confirmProductAssetUpload;
  });
}

// 販売ファイルを1つ削除する(Blob上のファイルも消える)。
export async function removeProductFile(assetId: number): Promise<ActionResult> {
  return run(async () => {
    await sellerGraphql(`mutation RemoveProductFile($id: Int!) { deleteProductAsset(assetId: $id) }`, { id: assetId });
    refresh();
    return null;
  });
}
