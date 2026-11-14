"use client";

import { useTranslations } from "next-intl";
import { LocalizedPrice } from "@/components/currency/localized-price";
import { CloseIcon, MinusIcon, PlusIcon } from "../ui/icons";
import { CheckoutButton } from "../shop/checkout-button";
import { useCart } from "./cart-provider";

export function CartDrawer() {
  const t = useTranslations("Cart");
  const { items, total, isOpen, changeQuantity, closeCart } = useCart();
  return (
    <>
      <button className={`cart-scrim ${isOpen ? "is-open" : ""}`} onClick={closeCart} aria-label="Close cart" tabIndex={isOpen ? 0 : -1} />
      <aside className={`cart-drawer ${isOpen ? "is-open" : ""}`} aria-hidden={!isOpen} aria-label={t("cart")}>
        <div className="cart-heading"><h2>{t("cart")}</h2><button className="icon-button" onClick={closeCart} aria-label="Close"><CloseIcon /></button></div>
        {items.length === 0 ? (
          <div className="cart-empty"><p>{t("empty")}</p></div>
        ) : (
          <div className="cart-content">
            <ul className="cart-items">
              {items.map((item) => (
                <li key={item.id}>
                  <div className="cart-thumb" style={item.imageUrl ? { backgroundImage: `url(${item.imageUrl})` } : undefined} />
                  <div><strong>{item.name}</strong><div className="quantity-control">
                    <button onClick={() => changeQuantity(item.id, -1)} aria-label="Decrease"><MinusIcon /></button><span>{item.quantity}</span><button onClick={() => changeQuantity(item.id, 1)} aria-label="Increase"><PlusIcon /></button>
                  </div></div>
                  <span className="cart-line-price"><LocalizedPrice jpy={item.price * item.quantity} /></span>
                </li>
              ))}
            </ul>
            <div className="cart-total"><span>{t("total")}</span><strong><LocalizedPrice jpy={total} /></strong></div>
            <CheckoutButton />
          </div>
        )}
      </aside>
    </>
  );
}
