"use client";

import { useState } from "react";

export function NewsletterForm() {
  const [submitted, setSubmitted] = useState(false);
  return (
    <form className="newsletter-form" onSubmit={(event) => { event.preventDefault(); setSubmitted(true); }}>
      <label htmlFor="newsletter-email">メールアドレス</label>
      <div>
        <input id="newsletter-email" name="email" type="email" placeholder="you@example.com" required disabled={submitted} />
        <button type="submit" disabled={submitted}>{submitted ? "登録済み" : "登録する"}</button>
      </div>
      <small>{submitted ? "ありがとうございます。次のお便りをお楽しみに。" : "登録すると、プライバシーポリシーに同意したものとみなされます。"}</small>
    </form>
  );
}
