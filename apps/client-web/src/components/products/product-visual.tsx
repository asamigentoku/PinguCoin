import type { Product } from "@/lib/types";

export function ProductVisual({ product }: { product: Product }) {
  return (
    <div className={`product-visual visual-${product.categoryId % 4}`} style={product.imageUrl ? { backgroundImage: `url(${product.imageUrl})` } : undefined}>
      {!product.imageUrl && <span>{product.name.slice(0, 1).toUpperCase()}</span>}
    </div>
  );
}
