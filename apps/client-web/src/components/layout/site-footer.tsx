import Link from "next/link";
import { mainNav } from "@/lib/navigation";
import { Brand } from "./brand";

export function SiteFooter() {
  return (
    <footer className="site-footer">
      <div className="shell site-footer-inner">
        <div><Brand /><p>好きから始まる、小さな経済圏。</p></div>
        <nav aria-label="フッターメニュー">{mainNav.map((item) => <Link key={item.href} href={item.href}>{item.label}</Link>)}</nav>
        <small>© 2026 PinguCoin</small>
      </div>
    </footer>
  );
}
