import { categories } from "./categories";

// 出品(商品)の状態。storefrontには "draft" 以外だけが並ぶ。
export const listingStatuses = [
  { value: "available", label: "公開中" },
  { value: "draft", label: "下書き" },
] as const;
export type ListingStatus = (typeof listingStatuses)[number]["value"];

export function statusLabel(status: string) {
  return listingStatuses.find((item) => item.value === status)?.label ?? status;
}

export type ListingInput = { name: string; description: string; categoryId: number; price: number; status: ListingStatus };

// 販売するファイルの用途ID(orcan-apiの product_asset_purposes: 3 = product_file、非公開)。
export const PRODUCT_FILE_PURPOSE_ID = 3;

export function formatBytes(bytes: number) {
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
  return bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.ceil(bytes / 1024))} KB`;
}

export const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
export const MAX_FILE_BYTES = 200 * 1024 * 1024; // 1ファイルあたり。ファイルの数に上限は無い。

export function validateListing(input: ListingInput): string | null {
  if (!input.name.trim()) return "商品名を入力してください。";
  if (input.name.trim().length > 100) return "商品名は100文字以内にしてください。";
  if (!Number.isInteger(input.price) || input.price < 0 || input.price > 10_000_000) return "価格は0〜10,000,000ポイントの整数で入力してください。";
  if (!categories.some((category) => category.id === input.categoryId)) return "カテゴリーを選んでください。";
  if (!listingStatuses.some((item) => item.value === input.status)) return "公開状態が正しくありません。";
  return null;
}

// Blob名に使えるよう、ファイル名をASCIIのUUIDに置き換える(拡張子は残す)。
export function safeBlobName(filename: string) {
  const dot = filename.lastIndexOf(".");
  const ext = dot > 0 ? filename.slice(dot).replace(/[^A-Za-z0-9.]/g, "").slice(0, 10) : "";
  return `${crypto.randomUUID()}${ext}`;
}
