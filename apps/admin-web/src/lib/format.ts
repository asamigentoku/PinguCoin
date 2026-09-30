export function formatPrice(value: number) {
  return new Intl.NumberFormat("ja-JP", { style: "currency", currency: "JPY", maximumFractionDigits: 0 }).format(value);
}

export function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("ja-JP", { dateStyle: "medium" }).format(date);
}

export const categoryNames: Record<number, string> = { 1: "アート", 2: "テンプレート", 3: "音楽", 4: "ツール" };
