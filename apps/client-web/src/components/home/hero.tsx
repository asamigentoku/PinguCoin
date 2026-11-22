import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";

export function Hero() {
  const t = useTranslations("Home");
  return (
    <section className="hero shell" aria-labelledby="hero-title">
      <div className="hero-copy">
        <h1 id="hero-title">{t("title")}</h1>
        <p>{t("lead")}</p>
        <div className="hero-actions">
          <Link className="button button-coin" href="/products">{t("browse")}</Link>
          <Link className="text-link" href="/about">{t("about")}</Link>
        </div>
      </div>
      <div className="hero-art" aria-hidden="true">
        <div className="hero-coin"><div className="penguin"><span className="eye eye-left" /><span className="eye eye-right" /><span className="beak" /></div></div>
        <span className="float-coin float-one">¥</span><span className="float-coin float-two">P</span>
      </div>
    </section>
  );
}
