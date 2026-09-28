"use client";

import type { Product } from "@/lib/types";
import { useCart } from "./cart-provider";
import { BagIcon } from "./icons";

export function ProductActions({ product }: { product: Product }) {
  const { addItem } = useCart();
  return <button className="button button-primary product-add" onClick={() => addItem(product)}><BagIcon /> カートに追加する</button>;
}
