import Link from "next/link";
import { categoryName } from "@/lib/categories";
import { formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";
import { AddToCartButton } from "./add-to-cart-button";
import { FavoriteButton } from "./favorite-button";
import { ProductVisual } from "./product-visual";

export function ProductCard({ product }: { product: Product }) {
  return (
    <article className="product-card">
      <div className="product-image-wrap">
        <Link href={`/products/${product.id}`} className="product-image-link" aria-label={`${product.name}の詳細を見る`}><ProductVisual product={product} /></Link>
        <FavoriteButton name={product.name} />
        <span className="price-coin">{formatPrice(product.price)}</span>
      </div>
      <div className="product-meta">
        <p>{categoryName(product.categoryId)}</p>
        <Link href={`/products/${product.id}`}>{product.name}</Link>
      </div>
      <AddToCartButton product={product} compact />
    </article>
  );
}
