"use client";

import { useCart } from "../cart/cart-provider";
import { BagIcon } from "../ui/icons";

export function CartButton() {
  const { count, openCart } = useCart();
  return <button className="cart-button" onClick={openCart} aria-label={`カート、${count}点`}><BagIcon /><b>{count}</b></button>;
}
