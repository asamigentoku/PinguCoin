"use client";

import { useQuery } from "@tanstack/react-query";
import { useLocale } from "next-intl";

const targetByLocale = { en: "en", fr: "fr", de: "de", zh: "zh-CN" } as const;
type TranslationLocale = keyof typeof targetByLocale;
type TranslationResponse = { translatedText: string };

export function useTranslatedText(japaneseText: string) {
  const locale = useLocale();
  const target = targetByLocale[locale as TranslationLocale];
  const query = useQuery({
    queryKey: ["translation", "ja", target, japaneseText],
    enabled: Boolean(target && japaneseText.trim()),
    queryFn: async ({ signal }) => {
      const params = new URLSearchParams({ text: japaneseText, target });
      const response = await fetch(`/api/translations?${params}`, { signal });
      if (!response.ok) throw new Error(`Translation API returned ${response.status}`);
      return response.json() as Promise<TranslationResponse>;
    },
  });

  return query.data?.translatedText?.trim() || japaneseText;
}

export function TranslatedText({ children }: { children: string }) {
  return <>{useTranslatedText(children)}</>;
}
