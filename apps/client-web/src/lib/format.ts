import { formatPoints } from "./points";

// 価格はすべてポイント。
export function formatPrice(value: number) {
  return `${formatPoints(value)} pt`;
}
