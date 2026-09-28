"use client";

import { createContext, useContext, useMemo, useState } from "react";
import type { CartItem, Product } from "@/lib/types";
import { CloseIcon, MinusIcon, PlusIcon } from "./icons";

type CartContextValue = { count: number; addItem: (product: Product) => void; openCart: () => void };
const CartContext = createContext<CartContextValue | null>(null);

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const value = useMemo<CartContextValue>(() => ({
    count: items.reduce((sum, item) => sum + item.quantity, 0),
    addItem: (product) => {
      setItems((current) => {
        const existing = current.find((item) => item.id === product.id);
        if (existing) return current.map((item) => item.id === product.id ? { ...item, quantity: item.quantity + 1 } : item);
        return [...current, { id: product.id, name: product.name, price: product.price, imageUrl: product.imageUrl, quantity: 1 }];
      });
      setIsOpen(true);
    },
    openCart: () => setIsOpen(true),
  }), [items]);
  const total = items.reduce((sum, item) => sum + item.price * item.quantity, 0);
  const changeQuantity = (id: number, amount: number) => setItems((current) => current.flatMap((item) => item.id !== id ? [item] : item.quantity + amount > 0 ? [{ ...item, quantity: item.quantity + amount }] : []));

  return (
    <CartContext.Provider value={value}>
      {children}
      <button className={`cart-scrim ${isOpen ? "is-open" : ""}`} onClick={() => setIsOpen(false)} aria-label="カートを閉じる" />
      <aside className={`cart-drawer ${isOpen ? "is-open" : ""}`} aria-hidden={!isOpen}>
        <div className="cart-heading"><div><p className="eyebrow">YOUR SELECTION</p><h2>カート</h2></div><button className="icon-button" onClick={() => setIsOpen(false)} aria-label="閉じる"><CloseIcon /></button></div>
        {items.length === 0 ? <div className="cart-empty"><span>○</span><p>カートはまだ空です。</p><small>お気に入りの作品を見つけてください。</small></div> : (
          <div className="cart-content">
            <ul className="cart-items">{items.map((item) => <li key={item.id}><div className="cart-thumb" style={item.imageUrl ? { backgroundImage: `url(${item.imageUrl})` } : undefined} /><div><strong>{item.name}</strong><div className="quantity-control"><button onClick={() => changeQuantity(item.id, -1)} aria-label={`${item.name}を1点減らす`}><MinusIcon /></button><span>{item.quantity}</span><button onClick={() => changeQuantity(item.id, 1)} aria-label={`${item.name}を1点増やす`}><PlusIcon /></button></div></div><span>{formatPrice(item.price * item.quantity)}</span></li>)}</ul>
            <div className="cart-total"><span>合計</span><strong>{formatPrice(total)}</strong></div>
            <button className="button button-primary checkout-button" disabled>チェックアウト準備中</button>
            <p className="cart-caption">認証・注文機能の接続後にご利用いただけます。</p>
          </div>
        )}
      </aside>
    </CartContext.Provider>
  );
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}

function formatPrice(value: number) { return new Intl.NumberFormat("ja-JP", { style: "currency", currency: "JPY" }).format(value); }
