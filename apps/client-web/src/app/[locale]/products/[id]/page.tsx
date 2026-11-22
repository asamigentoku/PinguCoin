import { notFound } from "next/navigation";
import { connection } from "next/server";
import { getTranslations } from "next-intl/server";
import { EmptyState } from "@/components/common/empty-state";
import { LocalizedPrice } from "@/components/currency/localized-price";
import { AddToCartButton } from "@/components/products/add-to-cart-button";
import { ProductVisual } from "@/components/products/product-visual";
import { TranslatedText } from "@/components/translation/translated-text";
import { Link } from "@/i18n/navigation";
import { getProduct } from "@/lib/api";
import { categoryName } from "@/lib/categories";
import { productsHref } from "@/lib/products";

const categoryKeys = ["art", "templates", "music", "tools"] as const;

export default async function ProductPage({ params }: PageProps<"/[locale]/products/[id]">) {
  await connection();
  const { id: rawId } = await params;
  const id = Number(rawId);
  if (!Number.isInteger(id) || id < 1) notFound();
  const t = await getTranslations("Products");

  let product;
  try { product = await getProduct(id); }
  catch {
    return <div className="shell"><EmptyState title={t("unavailable")} body={t("tryAgain")} href="/products" action={t("back")} /></div>;
  }
  if (!product) notFound();
  const categoryKey = categoryKeys[product.categoryId - 1];
  const category = categoryKey ? t(categoryKey) : categoryName(product.categoryId);

  return (
    <div className="shell product-page">
      <nav className="breadcrumb" aria-label="Breadcrumb"><Link href="/products">{t("title")}</Link><span aria-hidden="true">/</span><Link href={productsHref({ category: product.categoryId })}>{category}</Link></nav>
      <div className="product-detail">
        <div className="detail-visual"><ProductVisual product={product} /></div>
        <div className="detail-copy">
          <p className="detail-category">{category}</p><h1><TranslatedText>{product.name}</TranslatedText></h1>
          <p className="detail-price"><LocalizedPrice jpy={product.price} showSettlement /></p>
          <p className="detail-description">{product.description || t("defaultDescription")}</p>
          <AddToCartButton product={product} />
          <dl className="detail-facts">
            <div><dt>{t("formatLabel")}</dt><dd>{t("formatValue")}</dd></div>
            <div><dt>{t("deliveryLabel")}</dt><dd>{t("deliveryValue")}</dd></div>
            <div><dt>{t("statusLabel")}</dt><dd>{product.status || "available"}</dd></div>
          </dl>
        </div>
      </div>
    </div>
  );
}
