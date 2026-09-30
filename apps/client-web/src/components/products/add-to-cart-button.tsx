"use client";

import type { Product } from "@/lib/types";
import { useCart } from "../cart/cart-provider";
import { BagIcon } from "../ui/icons";

export function AddToCartButton({ product, compact = false }: { product: Product; compact?: boolean }) {
  const { addItem } = useCart();
  return compact
    ? <button className="quick-add" onClick={() => addItem(product)}>カートに追加</button>
    : <button className="button button-primary product-add" onClick={() => addItem(product)}><BagIcon /> カートに追加する</button>;
}
