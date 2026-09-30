"use client";

import { Show, SignInButton } from "@clerk/nextjs";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { purchase } from "@/app/purchases/actions";
import { formatPrice } from "@/lib/format";
import { useCart } from "../cart/cart-provider";

export function CheckoutButton() {
  const router = useRouter();
  const { items, total, removeItems, closeCart } = useCart();
  const dialog = useRef<HTMLDialogElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  function openConfirm() {
    setError("");
    dialog.current?.showModal();
  }

  async function confirmPurchase() {
    setBusy(true);
    setError("");
    // 押すたびに新しい購入操作として扱う。処理中はボタンを無効にして二重クリックを防ぐ。
    const result = await purchase(items.map((item) => ({ productId: item.id, quantity: item.quantity })), crypto.randomUUID());
    removeItems(result.purchased);
    setBusy(false);
    if (result.error) return setError(result.error);
    dialog.current?.close();
    closeCart();
    router.push("/purchases");
    router.refresh();
  }

  return (
    <>
      <Show when="signed-in">
        <button className="button button-primary checkout-button" onClick={openConfirm}>ポイントで購入する</button>
      </Show>
      <Show when="signed-out">
        <SignInButton><button className="button button-primary checkout-button">ログインして購入する</button></SignInButton>
      </Show>

      <dialog ref={dialog} className="confirm-dialog" aria-labelledby="confirm-title" onCancel={(e) => busy && e.preventDefault()}>
        <h2 id="confirm-title">この内容で購入しますか？</h2>
        <ul className="confirm-items">
          {items.map((item) => (
            <li key={item.id}>
              <span>{item.name}{item.quantity > 1 && ` × ${item.quantity}`}</span>
              <strong>{formatPrice(item.price * item.quantity)}</strong>
            </li>
          ))}
        </ul>
        <div className="confirm-total"><span>合計</span><strong>{formatPrice(total)}</strong></div>
        <p className="confirm-note">保有ポイントから引き落とされます。購入後は、購入履歴からダウンロードできます。</p>
        {error && <p className="form-error" role="alert">{error}</p>}
        <div className="confirm-actions">
          <button className="button button-ghost" onClick={() => dialog.current?.close()} disabled={busy}>キャンセル</button>
          <button className="button button-coin" onClick={confirmPurchase} disabled={busy || items.length === 0}>{busy ? "購入しています…" : "購入を確定する"}</button>
        </div>
      </dialog>
    </>
  );
}
