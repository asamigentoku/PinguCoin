export type NavLink = { href: string; label: string };
export type NavGroup = { label: string; children: NavLink[] };
export type NavItem = NavLink | NavGroup;

export const mainNav: NavItem[] = [
  { href: "/", label: "ホーム" },
  { href: "/products", label: "商品を探す" },
  { href: "/sell", label: "商品を出品する" },
  { href: "/points", label: "ポイントをためる" },
  {
    label: "PinguCoin",
    children: [
      { href: "/about", label: "PinguCoinについて" },
      { href: "/guide", label: "ご利用ガイド" },
      { href: "/contact", label: "お問い合わせ" },
    ],
  },
];

// フッターなど、グループを使わない箇所向けの平らな一覧。
export const flatNav: NavLink[] = mainNav.flatMap((item) => ("children" in item ? item.children : [item]));

export function isActivePath(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);
}
