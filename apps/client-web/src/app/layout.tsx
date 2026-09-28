import type { Metadata } from "next";
import Link from "next/link";
import { CartProvider } from "@/components/cart-provider";
import { SiteHeader } from "@/components/site-header";
import "./globals.css";

export const metadata: Metadata = {
  title: "PinguCoin — 好きの価値を、もっと近くに。",
  description: "クリエイターのデジタルプロダクトと出会えるマーケットプレイス。",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ja"><body><CartProvider><SiteHeader />{children}</CartProvider>
      <footer className="site-footer shell">
        <Link className="brand brand-footer" href="/" aria-label="PinguCoin ホーム"><span className="brand-mark">P</span><span>PINGUCOIN</span></Link>
        <p>好きから始まる、小さな経済圏。</p>
        <div><a href="#catalog">商品一覧</a><a href="#about">PinguCoinについて</a><a href="#">ご利用ガイド</a><a href="#">お問い合わせ</a><span>© 2026 PINGUCOIN</span></div>
      </footer>
    </body></html>
  );
}
