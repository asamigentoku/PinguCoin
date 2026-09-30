import type { Metadata } from "next";
import { AdminNav } from "@/components/admin-nav";
import "./globals.css";

export const metadata: Metadata = {
  title: "PinguCoin Admin",
  description: "PinguCoinの商品・ユーザーを管理する管理画面です。",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ja">
      <body>
        <AdminNav />
        {children}
      </body>
    </html>
  );
}
