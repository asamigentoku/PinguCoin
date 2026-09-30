import Link from "next/link";
import type { Product } from "@/lib/types";
import { EmptyState } from "../common/empty-state";
import { ProductGrid } from "../products/product-grid";

export function LatestProducts({ products, apiAvailable }: { products: Product[]; apiAvailable: boolean }) {
  return (
    <section className="home-section shell" aria-labelledby="latest-title">
      <div className="section-heading"><h2 id="latest-title">新着の作品</h2><Link className="text-link" href="/products">もっと見る</Link></div>
      {!apiAvailable
        ? <EmptyState title="商品を読み込めませんでした" body="pingu-apiが起動しているか確認して、ページを再読み込みしてください。" />
        : products.length === 0
          ? <EmptyState title="作品はまだありません" body="商品が登録されると、ここに新着として並びます。" />
          : <ProductGrid products={products} />}
    </section>
  );
}
