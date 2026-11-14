"use client";

import { useTranslations } from "next-intl";
import { useCart } from "../cart/cart-provider";
import { BagIcon } from "../ui/icons";

export function CartButton() {
  const t = useTranslations("Cart");
  const { count, openCart } = useCart();
  return <button className="cart-button" onClick={openCart} aria-label={`${t("cart")} (${count})`}><BagIcon /><b>{count}</b></button>;
}
