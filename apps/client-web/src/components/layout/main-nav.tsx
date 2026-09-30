"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { isActivePath, mainNav } from "@/lib/navigation";

export function MainNav() {
  const pathname = usePathname();
  return (
    <nav className="main-nav" aria-label="メインメニュー">
      {mainNav.map((item) => {
        const active = isActivePath(pathname, item.href);
        return (
          <Link key={item.href} href={item.href} className={active ? "is-active" : undefined} aria-current={active ? "page" : undefined}>
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
