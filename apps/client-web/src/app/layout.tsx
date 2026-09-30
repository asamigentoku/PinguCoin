import { ClerkProvider } from "@clerk/nextjs";
import type { Metadata } from "next";
import { CartProvider } from "@/components/cart/cart-provider";
import { SellFab } from "@/components/layout/sell-fab";
import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "PinguCoin — 好きの価値を、もっと近くに。", template: "%s | PinguCoin" },
  description: "クリエイターのデジタルプロダクトと出会えるマーケットプレイス。",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ja">
      <body>
        <ClerkProvider>
          <CartProvider>
            <SiteHeader />
            <main className="site-main">{children}</main>
            <SiteFooter />
            <SellFab />
          </CartProvider>
        </ClerkProvider>
      </body>
    </html>
  );
}