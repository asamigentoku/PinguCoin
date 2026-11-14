"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRef } from "react";
import { usePathname, useRouter } from "@/i18n/navigation";
import { routing, type AppLocale } from "@/i18n/routing";

const localeOptions: Record<AppLocale, { label: string; flagLabel: string }> = {
  ja: { label: "日本語", flagLabel: "日本" },
  en: { label: "English", flagLabel: "United States" },
  fr: { label: "Français", flagLabel: "France" },
  de: { label: "Deutsch", flagLabel: "Deutschland" },
  zh: { label: "中文", flagLabel: "中国" },
};

function Flag({ locale }: { locale: AppLocale }) {
  const common = { className: "locale-flag", role: "img" as const, "aria-label": localeOptions[locale].flagLabel, viewBox: "0 0 24 16" };
  if (locale === "ja") return <svg {...common}><rect width="24" height="16" fill="#fff" /><circle cx="12" cy="8" r="4.4" fill="#bc002d" /></svg>;
  if (locale === "fr") return <svg {...common}><rect width="8" height="16" fill="#0055a4" /><rect x="8" width="8" height="16" fill="#fff" /><rect x="16" width="8" height="16" fill="#ef4135" /></svg>;
  if (locale === "de") return <svg {...common}><rect width="24" height="5.34" fill="#000" /><rect y="5.33" width="24" height="5.34" fill="#dd0000" /><rect y="10.66" width="24" height="5.34" fill="#ffce00" /></svg>;
  if (locale === "zh") return <svg {...common}><rect width="24" height="16" fill="#de2910" /><path d="M4 2.2 4.7 4.3H7L5.1 5.6l.7 2.2L4 6.5 2.2 7.8l.7-2.2L1 4.3h2.3Z" fill="#ffde00" /></svg>;
  return (
    <svg {...common}>
      <rect width="24" height="16" fill="#fff" />
      {[0, 2.46, 4.92, 7.38, 9.84, 12.3, 14.76].map((y) => <rect key={y} y={y} width="24" height="1.23" fill="#b22234" />)}
      <rect width="10.5" height="8.6" fill="#3c3b6e" />
      {[2, 5.2, 8.4].flatMap((x) => [2, 4.3, 6.6].map((y) => <circle key={`${x}-${y}`} cx={x} cy={y} r=".45" fill="#fff" />))}
    </svg>
  );
}

export function LocaleSwitcher() {
  const locale = useLocale() as AppLocale;
  const t = useTranslations("Navigation");
  const pathname = usePathname();
  const router = useRouter();
  const detailsRef = useRef<HTMLDetailsElement>(null);

  function changeLocale(nextLocale: AppLocale) {
    detailsRef.current?.removeAttribute("open");
    if (nextLocale !== locale) router.replace(pathname, { locale: nextLocale });
  }

  return (
    <details ref={detailsRef} className="locale-switcher">
      <summary aria-label={t("language")}><Flag locale={locale} /><span>{localeOptions[locale].label}</span><span className="locale-chevron" aria-hidden="true">▾</span></summary>
      <ul className="locale-menu">
        {routing.locales.map((value) => (
          <li key={value}>
            <button type="button" aria-current={value === locale ? "true" : undefined} onClick={() => changeLocale(value)}>
              <Flag locale={value} /><span>{localeOptions[value].label}</span>
            </button>
          </li>
        ))}
      </ul>
    </details>
  );
}
