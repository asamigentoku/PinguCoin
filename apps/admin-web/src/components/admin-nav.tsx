"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links: Array<{ href: "/products" | "/users"; label: string }> = [
  { href: "/products", label: "商品管理" },
  { href: "/users", label: "ユーザー管理" },
];

export function AdminNav() {
  const pathname = usePathname();
  return (
    <header className="admin-header">
      <div className="admin-header-inner">
        <Link className="admin-brand" href="/" aria-label="PinguCoin管理画面 ホーム">
          <span className="admin-brand-mark">P</span>
          <span>PINGUCOIN<b>ADMIN</b></span>
        </Link>
        <nav className="admin-nav" aria-label="管理メニュー">
          {links.map((link) => {
            const active = pathname === link.href || pathname.startsWith(`${link.href}/`);
            return (
              <Link key={link.href} href={link.href} className={active ? "is-active" : ""} aria-current={active ? "page" : undefined}>
                {link.label}
              </Link>
            );
          })}
        </nav>
      </div>
    </header>
  );
}
