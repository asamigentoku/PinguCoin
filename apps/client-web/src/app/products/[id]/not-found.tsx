import { EmptyState } from "@/components/common/empty-state";

export default function ProductNotFound() {
  return (
    <div className="shell">
      <EmptyState title="この商品は見つかりませんでした" body="販売が終了したか、URLが変更された可能性があります。" href="/products" action="商品一覧へ戻る" />
    </div>
  );
}
