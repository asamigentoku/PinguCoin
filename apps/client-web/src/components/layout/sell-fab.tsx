"use client";

import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import { PlusIcon } from "../ui/icons";

export function SellFab() {
  const t = useTranslations("Navigation");
  const pathname = usePathname();
  if (pathname.startsWith("/sell") || pathname.startsWith("/sign-")) return null;
  return <Link className="sell-fab" href="/sell"><PlusIcon size={18} />{t("sell")}</Link>;
}
