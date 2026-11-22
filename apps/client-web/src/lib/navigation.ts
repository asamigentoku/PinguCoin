export type NavigationKey = "home" | "products" | "sell" | "points" | "about" | "guide" | "contact";
export type NavLink = { href: string; labelKey: NavigationKey };
export type NavGroup = { label: string; children: NavLink[] };
export type NavItem = NavLink | NavGroup;

export const mainNav: NavItem[] = [
  { href: "/", labelKey: "home" },
  { href: "/products", labelKey: "products" },
  { href: "/sell", labelKey: "sell" },
  { href: "/points", labelKey: "points" },
  {
    label: "PinguCoin",
    children: [
      { href: "/about", labelKey: "about" },
      { href: "/guide", labelKey: "guide" },
      { href: "/contact", labelKey: "contact" },
    ],
  },
];

export const flatNav: NavLink[] = mainNav.flatMap((item) => ("children" in item ? item.children : [item]));

export function isActivePath(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);
}
