import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import type { Product } from "@/lib/types";
import { EmptyState } from "../common/empty-state";
import { ProductGrid } from "../products/product-grid";

export function LatestProducts({ products, apiAvailable }: { products: Product[]; apiAvailable: boolean }) {
  const t = useTranslations("Home");
  return (
    <section className="home-section shell" aria-labelledby="latest-title">
      <div className="section-heading"><h2 id="latest-title">{t("latest")}</h2><Link className="text-link" href="/products">{t("more")}</Link></div>
      {!apiAvailable
        ? <EmptyState title={t("loadErrorTitle")} body={t("loadErrorBody")} />
        : products.length === 0
          ? <EmptyState title={t("emptyTitle")} body={t("emptyBody")} />
          : <ProductGrid products={products} />}
    </section>
  );
}
