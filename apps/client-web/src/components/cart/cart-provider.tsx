"use client";

import { createContext, useContext, useMemo, useState } from "react";
import type { CartItem, Product } from "@/lib/types";
import { CartDrawer } from "./cart-drawer";

type CartContextValue = {
  items: CartItem[];
  count: number;
  total: number;
  isOpen: boolean;
  addItem: (product: Product) => void;
  changeQuantity: (id: number, amount: number) => void;
  removeItems: (ids: number[]) => void;
  openCart: () => void;
  closeCart: () => void;
};
const CartContext = createContext<CartContextValue | null>(null);

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [isOpen, setIsOpen] = useState(false);

  const value = useMemo<CartContextValue>(() => ({
    items,
    isOpen,
    count: items.reduce((sum, item) => sum + item.quantity, 0),
    total: items.reduce((sum, item) => sum + item.price * item.quantity, 0),
    addItem: (product) => {
      setItems((current) => current.some((item) => item.id === product.id)
        ? current.map((item) => (item.id === product.id ? { ...item, quantity: item.quantity + 1 } : item))
        : [...current, { id: product.id, name: product.name, price: product.price, imageUrl: product.imageUrl, quantity: 1 }]);
      setIsOpen(true);
    },
    changeQuantity: (id, amount) => setItems((current) => current.flatMap((item) =>
      item.id !== id ? [item] : item.quantity + amount > 0 ? [{ ...item, quantity: item.quantity + amount }] : [])),
    removeItems: (ids) => setItems((current) => current.filter((item) => !ids.includes(item.id))),
    openCart: () => setIsOpen(true),
    closeCart: () => setIsOpen(false),
  }), [items, isOpen]);

  return (
    <CartContext.Provider value={value}>
      {children}
      <CartDrawer />
    </CartContext.Provider>
  );
}

export function useCart() {
  const context = useContext(CartContext);
  if (!context) throw new Error("useCart must be used inside CartProvider");
  return context;
}
