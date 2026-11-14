import { ClerkProvider } from "@clerk/nextjs";
import type { Metadata } from "next";
import { hasLocale, NextIntlClientProvider } from "next-intl";
import { getMessages } from "next-intl/server";
import { notFound } from "next/navigation";
import { CartProvider } from "@/components/cart/cart-provider";
import { CurrencyProvider } from "@/components/currency/currency-provider";
import { SellFab } from "@/components/layout/sell-fab";
import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";
import { QueryProvider } from "@/components/query/query-provider";
import { routing } from "@/i18n/routing";
import { getExchangeRates } from "@/lib/api";
import "../globals.css";

export const metadata: Metadata = {
  title: { default: "PinguCoin — 好きの価値を、もっと近くに。", template: "%s | PinguCoin" },
  description: "クリエイターのデジタルプロダクトと出会えるマーケットプレイス。",
};

export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

export default async function RootLayout({ children, params }: { children: React.ReactNode; params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!hasLocale(routing.locales, locale)) notFound();
  const messages = await getMessages();
  const exchangeRates = await getExchangeRates().catch(() => null);
  return (
    <html lang={locale}>
      <body>
        <NextIntlClientProvider messages={messages}>
          <QueryProvider>
            <ClerkProvider>
              <CurrencyProvider initialData={exchangeRates}>
                <CartProvider>
                  <SiteHeader />
                  <main className="site-main">{children}</main>
                  <SiteFooter />
                  <SellFab />
                </CartProvider>
              </CurrencyProvider>
            </ClerkProvider>
          </QueryProvider>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
