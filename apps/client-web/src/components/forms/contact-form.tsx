"use client";

import { useState } from "react";

export function ContactForm() {
  const [sent, setSent] = useState(false);
  if (sent) return <p className="form-done" role="status">送信しました。内容を確認し、2営業日以内にご連絡します。</p>;
  return (
    <form className="contact-form" onSubmit={(event) => { event.preventDefault(); setSent(true); }}>
      <label>お名前<input name="name" required autoComplete="name" /></label>
      <label>メールアドレス<input name="email" type="email" required autoComplete="email" /></label>
      <label>お問い合わせ内容<textarea name="message" rows={6} required /></label>
      <button className="button button-primary" type="submit">送信する</button>
    </form>
  );
}
