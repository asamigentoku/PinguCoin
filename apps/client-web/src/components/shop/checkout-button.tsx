"use client";

import { Show, SignInButton } from "@clerk/nextjs";
import { useLocale, useTranslations } from "next-intl";
import { useRef, useState } from "react";
import { purchase } from "@/app/purchases/actions";
import { useRouter } from "@/i18n/navigation";
import { formatJPY } from "@/lib/format";
import type { PaymentMethod } from "@/lib/shop";
import { useCart } from "../cart/cart-provider";

export function CheckoutButton() {
  const locale = useLocale();
  const t = useTranslations("Cart");
  const priceT = useTranslations("Price");
  const router = useRouter();
  const { items, total, removeItems, closeCart } = useCart();
  const dialog = useRef<HTMLDialogElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>(locale === "ja" ? "point" : "card");

  async function confirmPurchase() {
    setBusy(true); setError("");
    const result = await purchase(items.map((item) => ({ productId: item.id, quantity: item.quantity })), paymentMethod, crypto.randomUUID());
    removeItems(result.purchased); setBusy(false);
    if (result.actionUrl) { window.location.assign(result.actionUrl); return; }
    if (result.error) return setError(result.error);
    dialog.current?.close(); closeCart(); router.push("/purchases"); router.refresh();
  }

  return (
    <>
      <Show when="signed-in"><button className="button button-primary checkout-button" onClick={() => { setError(""); dialog.current?.showModal(); }}>{t("checkout")}</button></Show>
      <Show when="signed-out"><SignInButton><button className="button button-primary checkout-button">{t("checkout")}</button></SignInButton></Show>
      <dialog ref={dialog} className="confirm-dialog" aria-labelledby="confirm-title" onCancel={(event) => busy && event.preventDefault()}>
        <h2 id="confirm-title">{t("confirm")}</h2>
        <ul className="confirm-items">{items.map((item) => <li key={item.id}><span>{item.name}{item.quantity > 1 && ` × ${item.quantity}`}</span><strong>{formatJPY(item.price * item.quantity)}</strong></li>)}</ul>
        <div className="confirm-total"><span>{t("total")}</span><strong>{formatJPY(total)}</strong></div>
        <p className="confirm-note">{priceT("settlement")}</p>
        <fieldset className="payment-methods"><legend>{t("paymentMethod")}</legend>
          {locale === "ja" && <label><input type="radio" name="payment-method" checked={paymentMethod === "point"} onChange={() => setPaymentMethod("point")} /> {t("points")}</label>}
          <label><input type="radio" name="payment-method" checked={paymentMethod === "card"} onChange={() => setPaymentMethod("card")} /> {t("card")}</label>
          <label><input type="radio" name="payment-method" checked={paymentMethod === "bank_transfer"} onChange={() => setPaymentMethod("bank_transfer")} /> {t("bank")}</label>
        </fieldset>
        {error && <p className="form-error" role="alert">{error}</p>}
        <div className="confirm-actions"><button className="button button-ghost" onClick={() => dialog.current?.close()} disabled={busy}>{t("cancel")}</button><button className="button button-coin" onClick={confirmPurchase} disabled={busy || items.length === 0}>{busy ? t("processing") : t("purchase")}</button></div>
      </dialog>
    </>
  );
}
