import { defineRouting } from "next-intl/routing";

export const routing = defineRouting({
  locales: ["ja", "en", "fr", "de", "zh"],
  defaultLocale: "ja",
  localePrefix: "always",
});

export type AppLocale = (typeof routing.locales)[number];
