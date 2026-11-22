import { Show, SignInButton, SignUpButton, UserButton } from "@clerk/nextjs";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { SearchIcon } from "../ui/icons";
import { Brand } from "./brand";
import { CartButton } from "./cart-button";
import { LocaleSwitcher } from "./locale-switcher";
import { MainNav } from "./main-nav";
import { PointBalance } from "./point-balance";

export function SiteHeader() {
  const t = useTranslations("Navigation");
  return (
    <header className="site-header">
      <div className="site-header-inner shell">
        <Brand />
        <MainNav />
        <div className="header-actions">
          <PointBalance />
          <Link className="header-icon-link" href="/products" aria-label={t("search")}><SearchIcon /></Link>
          <Show when="signed-out">
            <SignInButton><button className="auth-link">{t("signIn")}</button></SignInButton>
            <SignUpButton><button className="auth-button">{t("signUp")}</button></SignUpButton>
          </Show>
          <Show when="signed-in">
            <Link className="header-manage-link" href="/purchases">{t("purchases")}</Link>
            <Link className="header-manage-link" href="/sell/manage">{t("manage")}</Link>
            <UserButton />
          </Show>
          <CartButton />
          <LocaleSwitcher />
        </div>
      </div>
    </header>
  );
}
