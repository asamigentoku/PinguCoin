"use client";

import { useLocale, useTranslations } from "next-intl";
import { formatPrice } from "@/lib/format";
import { useCurrency } from "./currency-provider";

const currencyByLocale = {
  en: { currency: "USD", numberLocale: "en-US" },
  fr: { currency: "EUR", numberLocale: "fr-FR" },
  de: { currency: "EUR", numberLocale: "de-DE" },
  zh: { currency: "CNY", numberLocale: "zh-CN" },
} as const;

export function LocalizedPrice({ jpy, showSettlement = false }: { jpy: number; showSettlement?: boolean }) {
  const locale = useLocale();
  const t = useTranslations("Price");
  const { rates, failed } = useCurrency();

  if (locale === "ja") return <>{formatPrice(jpy)}</>;
  const config = currencyByLocale[locale as keyof typeof currencyByLocale] ?? currencyByLocale.en;
  const jpyPerCurrency = rates[config.currency];
  const converted = jpyPerCurrency ? jpy / jpyPerCurrency : null;
  return (
    <span className="localized-price">
      <span>{converted ? new Intl.NumberFormat(config.numberLocale, { style: "currency", currency: config.currency }).format(converted) : failed ? t("unavailable") : t("loading")}</span>
      {showSettlement && <small>{t("settlement")}</small>}
    </span>
  );
}
