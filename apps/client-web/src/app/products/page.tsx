import type { Metadata } from "next";
import { connection } from "next/server";
import { EmptyState } from "@/components/common/empty-state";
import { PageHeader } from "@/components/common/page-header";
import { Pagination } from "@/components/products/pagination";
import { ProductGrid } from "@/components/products/product-grid";
import { ProductToolbar } from "@/components/products/product-toolbar";
import { getProducts } from "@/lib/api";
import { PAGE_SIZE, parseProductQuery, queryProducts } from "@/lib/products";
import type { Product } from "@/lib/types";

export const metadata: Metadata = { title: "商品を探す" };

export default async function ProductsPage({ searchParams }: PageProps<"/products">) {
  await connection();
  const query = parseProductQuery(await searchParams);
  let products: Product[] = [];
  let apiAvailable = true;
  try {
    products = await getProducts();
  } catch {
    apiAvailable = false;
  }
  const result = queryProducts(products, query);
  const from = (result.page - 1) * PAGE_SIZE + 1;
  const to = Math.min(result.page * PAGE_SIZE, result.total);

  return (
    <>
      <PageHeader title="商品を探す" lead="カテゴリーや検索で、お気に入りの作品を見つけましょう。" />
      <section className="shell products-section">
        <ProductToolbar query={query} />
        {!apiAvailable ? (
          <EmptyState title="商品を読み込めませんでした" body="pingu-apiが起動しているか確認して、ページを再読み込みしてください。" href="/products" action="再読み込みする" />
        ) : result.total === 0 ? (
          <EmptyState
            title={products.length ? "条件に合う作品がありません" : "作品はまだありません"}
            body={products.length ? "キーワードやカテゴリーを変えて、もう一度お試しください。" : "商品が登録されると、ここに並びます。"}
            href={products.length ? "/products" : undefined}
            action="条件をリセットする"
          />
        ) : (
          <>
            <p className="result-summary">{result.total}件中 {from}〜{to}件を表示</p>
            <ProductGrid products={result.items} />
            <Pagination query={query} page={result.page} pageCount={result.pageCount} />
          </>
        )}
      </section>
    </>
  );
}
