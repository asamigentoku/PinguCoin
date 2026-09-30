export type NavItem = { href: string; label: string };

export const mainNav: NavItem[] = [
  { href: "/", label: "ホーム" },
  { href: "/products", label: "商品を探す" },
  { href: "/about", label: "PinguCoinについて" },
  { href: "/guide", label: "ご利用ガイド" },
  { href: "/contact", label: "お問い合わせ" },
];

export function isActivePath(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);
}
