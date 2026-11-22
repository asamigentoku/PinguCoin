import type { Metadata } from "next";
import { connection } from "next/server";
import { getTranslations } from "next-intl/server";
import { EmptyState } from "@/components/common/empty-state";
import { PageHeader } from "@/components/common/page-header";
import { Pagination } from "@/components/products/pagination";
import { ProductGrid } from "@/components/products/product-grid";
import { ProductToolbar } from "@/components/products/product-toolbar";
import { getProducts } from "@/lib/api";
import { PAGE_SIZE, parseProductQuery, queryProducts } from "@/lib/products";
import type { Product } from "@/lib/types";

export const metadata: Metadata = { title: "商品を探す" };

export default async function ProductsPage({ searchParams }: PageProps<"/[locale]/products">) {
  await connection();
  const t = await getTranslations("Products");
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
      <PageHeader title={t("title")} lead={t("lead")} />
      <section className="shell products-section">
        <ProductToolbar query={query} />
        {!apiAvailable ? (
          <EmptyState title={t("loadErrorTitle")} body={t("tryAgain")} href="/products" action={t("reload")} />
        ) : result.total === 0 ? (
          <EmptyState
            title={products.length ? t("noMatchesTitle") : t("noCreationsTitle")}
            body={products.length ? t("noMatchesBody") : t("noCreationsBody")}
            href={products.length ? "/products" : undefined}
            action={t("reset")}
          />
        ) : (
          <>
            <p className="result-summary">{t("showing", { from, to, total: result.total })}</p>
            <ProductGrid products={result.items} />
            <Pagination query={query} page={result.page} pageCount={result.pageCount} />
          </>
        )}
      </section>
    </>
  );
}
