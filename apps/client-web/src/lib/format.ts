import { formatPoints } from "./points";

// 価格はすべてポイント。
export function formatPrice(value: number) {
  return `${formatPoints(value)} pt`;
}

export function formatJPY(value: number) {
  return new Intl.NumberFormat("ja-JP", { style: "currency", currency: "JPY", maximumFractionDigits: 0 }).format(value);
}
