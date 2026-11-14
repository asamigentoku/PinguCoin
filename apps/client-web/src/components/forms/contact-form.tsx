"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

export function ContactForm() {
  const t = useTranslations("Contact");
  const [sent, setSent] = useState(false);
  if (sent) return <p className="form-done" role="status">{t("sent")}</p>;
  return <form className="contact-form" onSubmit={(event) => { event.preventDefault(); setSent(true); }}><label>{t("name")}<input name="name" required autoComplete="name" /></label><label>Email<input name="email" type="email" required autoComplete="email" /></label><label>{t("message")}<textarea name="message" rows={6} required /></label><button className="button button-primary" type="submit">{t("send")}</button></form>;
}
