import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { flatNav } from "@/lib/navigation";
import { Brand } from "./brand";

export function SiteFooter() {
  const t = useTranslations("Navigation");
  return (
    <footer className="site-footer">
      <div className="shell site-footer-inner">
        <div><Brand /><p>{t("footerTagline")}</p></div>
        <nav aria-label="Footer">{flatNav.map((item) => <Link key={item.href} href={item.href}>{t(item.labelKey)}</Link>)}</nav>
        <small>© 2026 PinguCoin</small>
      </div>
    </footer>
  );
}
