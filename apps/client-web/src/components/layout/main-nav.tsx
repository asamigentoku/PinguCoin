"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { isActivePath, mainNav, type NavGroup } from "@/lib/navigation";

function NavDropdown({ group, pathname }: { group: NavGroup; pathname: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const active = group.children.some((child) => isActivePath(pathname, child.href));

  useEffect(() => {
    if (!open) return;
    const close = (e: Event) => {
      if (e instanceof KeyboardEvent ? e.key === "Escape" : !ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);

  return (
    <div className="nav-group" ref={ref}>
      <button type="button" className={active ? "is-active" : undefined} aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        {group.label}<span aria-hidden="true"> ▾</span>
      </button>
      {open && (
        <div className="nav-menu">
          {group.children.map((child) => (
            <Link key={child.href} href={child.href} aria-current={isActivePath(pathname, child.href) ? "page" : undefined} onClick={() => setOpen(false)}>{child.label}</Link>
          ))}
        </div>
      )}
    </div>
  );
}

export function MainNav() {
  const pathname = usePathname();
  return (
    <nav className="main-nav" aria-label="メインメニュー">
      {mainNav.map((item) => {
        if ("children" in item) return <NavDropdown key={item.label} group={item} pathname={pathname} />;
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
