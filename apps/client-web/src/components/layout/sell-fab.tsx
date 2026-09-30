"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { PlusIcon } from "../ui/icons";

// どのページからでも出品ページへ移動できる、画面右下の固定ボタン。出品関連のページでは隠す。
export function SellFab() {
  const pathname = usePathname();
  if (pathname.startsWith("/sell") || pathname.startsWith("/sign-")) return null;
  return <Link className="sell-fab" href="/sell"><PlusIcon size={18} />出品する</Link>;
}
