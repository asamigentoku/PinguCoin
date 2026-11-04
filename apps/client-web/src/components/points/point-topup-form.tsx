"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { topUpPoints } from "@/app/points/actions";
import type { ExternalPaymentMethod } from "@/lib/shop";

export function PointTopUpForm() {
  const router = useRouter();
  const [amount, setAmount] = useState(1000);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<ExternalPaymentMethod>("card");
  async function submit(event: React.FormEvent) {
    event.preventDefault(); setBusy(true); setError(""); setDone("");
    const result = await topUpPoints(amount, paymentMethod, crypto.randomUUID());
    setBusy(false);
    if (result.actionUrl) { window.location.assign(result.actionUrl); return; }
    if (result.error) setError(result.error);
    else { setDone(`${amount.toLocaleString("ja-JP")}ポイントをチャージしました。`); router.refresh(); }
  }
  return <form className="point-topup" onSubmit={submit}>
    <div><h2>ポイントをチャージ</h2><p>選択したモック決済で1円＝1ポイントを追加します。</p></div>
    <label>チャージ額<input type="number" min="100" max="100000" step="100" value={amount} onChange={(event) => setAmount(Number(event.target.value))} /> pt</label>
    <label>決済方法<select value={paymentMethod} onChange={(event) => setPaymentMethod(event.target.value as ExternalPaymentMethod)}><option value="card">カード</option><option value="bank_transfer">銀行振込</option></select></label>
    <button className="button button-primary" disabled={busy}>{busy ? "認証を準備中…" : "カードでチャージ"}</button>
    {error && <p className="form-error" role="alert">{error}</p>}
    {done && <p className="form-done" role="status">{done}</p>}
  </form>;
}
