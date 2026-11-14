"use client";

import { useTranslations } from "next-intl";
import type { Product } from "@/lib/types";
import { useTranslatedText } from "@/components/translation/translated-text";
import { useCart } from "../cart/cart-provider";
import { BagIcon } from "../ui/icons";

export function AddToCartButton({ product, compact = false }: { product: Product; compact?: boolean }) {
  const t = useTranslations("Products");
  const { addItem } = useCart();
  const translatedName = useTranslatedText(product.name);
  const localizedProduct = { ...product, name: translatedName };
  return compact
    ? <button className="quick-add" onClick={() => addItem(localizedProduct)}>{t("addToCart")}</button>
    : <button className="button button-primary product-add" onClick={() => addItem(localizedProduct)}><BagIcon /> {t("addToCart")}</button>;
}
