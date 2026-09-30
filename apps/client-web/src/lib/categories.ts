export type Category = { id: number; name: string; description: string; tone: string };

export const categories: Category[] = [
  { id: 1, name: "アート・イラスト", description: "壁紙、イラスト、素材集", tone: "tone-coral" },
  { id: 2, name: "テンプレート", description: "資料、Notion、デザイン雛形", tone: "tone-sea" },
  { id: 3, name: "音楽・サウンド", description: "BGM、効果音、サンプル", tone: "tone-violet" },
  { id: 4, name: "便利ツール", description: "作業を助ける小さな道具", tone: "tone-coin" },
];

export function categoryName(id: number) {
  return categories.find((category) => category.id === id)?.name ?? `カテゴリー ${id}`;
}
