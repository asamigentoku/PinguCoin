"use client";

import { Link } from "@/i18n/navigation";
import { useTranslations } from "next-intl";
import { LocalizedPrice } from "@/components/currency/localized-price";
import { categoryName } from "@/lib/categories";
import type { Product } from "@/lib/types";
import { useTranslatedText } from "@/components/translation/translated-text";
import { AddToCartButton } from "./add-to-cart-button";
import { FavoriteButton } from "./favorite-button";
import { ProductVisual } from "./product-visual";

export function ProductCard({ product }: { product: Product }) {
  const t = useTranslations("Products");
  const translatedName = useTranslatedText(product.name);
  const localizedProduct = { ...product, name: translatedName };
  const categoryKeys = ["art", "templates", "music", "tools"] as const;
  return (
    <article className="product-card">
      <div className="product-image-wrap">
        <Link href={`/products/${product.id}`} className="product-image-link" aria-label={translatedName}><ProductVisual product={localizedProduct} /></Link>
        <FavoriteButton name={translatedName} />
        <span className="price-coin"><LocalizedPrice jpy={product.price} /></span>
      </div>
      <div className="product-meta">
        <p>{categoryKeys[product.categoryId - 1] ? t(categoryKeys[product.categoryId - 1]) : categoryName(product.categoryId)}</p>
        <Link href={`/products/${product.id}`}>{translatedName}</Link>
      </div>
      <AddToCartButton product={product} compact />
    </article>
  );
}
