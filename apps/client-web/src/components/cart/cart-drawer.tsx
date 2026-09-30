"use client";

import { formatPrice } from "@/lib/format";
import { CloseIcon, MinusIcon, PlusIcon } from "../ui/icons";
import { useCart } from "./cart-provider";

export function CartDrawer() {
  const { items, total, isOpen, changeQuantity, closeCart } = useCart();
  return (
    <>
      <button className={`cart-scrim ${isOpen ? "is-open" : ""}`} onClick={closeCart} aria-label="カートを閉じる" tabIndex={isOpen ? 0 : -1} />
      <aside className={`cart-drawer ${isOpen ? "is-open" : ""}`} aria-hidden={!isOpen} aria-label="カート">
        <div className="cart-heading">
          <h2>カート</h2>
          <button className="icon-button" onClick={closeCart} aria-label="閉じる"><CloseIcon /></button>
        </div>
        {items.length === 0 ? (
          <div className="cart-empty"><p>カートは空です。</p><small>気になる作品を追加してみましょう。</small></div>
        ) : (
          <div className="cart-content">
            <ul className="cart-items">
              {items.map((item) => (
                <li key={item.id}>
                  <div className="cart-thumb" style={item.imageUrl ? { backgroundImage: `url(${item.imageUrl})` } : undefined} />
                  <div>
                    <strong>{item.name}</strong>
                    <div className="quantity-control">
                      <button onClick={() => changeQuantity(item.id, -1)} aria-label={`${item.name}を1点減らす`}><MinusIcon /></button>
                      <span>{item.quantity}</span>
                      <button onClick={() => changeQuantity(item.id, 1)} aria-label={`${item.name}を1点増やす`}><PlusIcon /></button>
                    </div>
                  </div>
                  <span className="cart-line-price">{formatPrice(item.price * item.quantity)}</span>
                </li>
              ))}
            </ul>
            <div className="cart-total"><span>合計</span><strong>{formatPrice(total)}</strong></div>
            <button className="button button-primary checkout-button" disabled>購入手続きは準備中です</button>
            <p className="cart-caption">ログインと注文機能の公開後にご利用いただけます。</p>
          </div>
        )}
      </aside>
    </>
  );
}
