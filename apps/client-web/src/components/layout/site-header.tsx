import Link from "next/link";
import { Show, SignInButton, SignUpButton, UserButton } from "@clerk/nextjs";
import { SearchIcon } from "../ui/icons";
import { Brand } from "./brand";
import { CartButton } from "./cart-button";
import { MainNav } from "./main-nav";

export function SiteHeader() {
  return (
    <header className="site-header">
      <div className="site-header-inner shell">
        <Brand />
        <MainNav />
        <div className="header-actions">
          <Link className="header-icon-link" href="/products" aria-label="商品を検索"><SearchIcon /></Link>
          <Show when="signed-out">
            <SignInButton><button className="auth-link">ログイン</button></SignInButton>
            <SignUpButton><button className="auth-button">新規登録</button></SignUpButton>
          </Show>
          <Show when="signed-in">
            <UserButton />
          </Show>
          <CartButton />
        </div>
      </div>
    </header>
  );
}
