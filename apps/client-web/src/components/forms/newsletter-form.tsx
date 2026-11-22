"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

export function NewsletterForm() {
  const t = useTranslations("Newsletter");
  const [submitted, setSubmitted] = useState(false);
  return <form className="newsletter-form" onSubmit={(event) => { event.preventDefault(); setSubmitted(true); }}><label htmlFor="newsletter-email">Email</label><div><input id="newsletter-email" name="email" type="email" placeholder="you@example.com" required disabled={submitted} /><button type="submit" disabled={submitted}>{submitted ? t("subscribed") : t("subscribe")}</button></div><small>{submitted ? t("thanks") : t("unsubscribe")}</small></form>;
}
