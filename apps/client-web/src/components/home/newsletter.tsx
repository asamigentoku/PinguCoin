import { useTranslations } from "next-intl";
import { NewsletterForm } from "../forms/newsletter-form";

export function Newsletter() {
  const t = useTranslations("Home");
  return <section className="home-section shell"><div className="newsletter"><div><h2>{t("newsletterTitle")}</h2><p>{t("newsletterBody")}</p></div><NewsletterForm /></div></section>;
}
