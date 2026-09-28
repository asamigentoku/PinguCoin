"use client";

import Link from "next/link";
import { useCart } from "./cart-provider";
import { BagIcon, SearchIcon, UserIcon } from "./icons";

export function SiteHeader() {
  const { count, openCart } = useCart();
  return <>
    <div className="promo-bar"><p>新着デジタル作品を毎週追加</p><span>安全な決済</span><span>購入後すぐダウンロード</span></div>
    <header className="site-header shell">
      <Link className="brand" href="/" aria-label="PinguCoin ホーム"><span className="brand-mark">P</span><span>PINGUCOIN</span></Link>
      <Link className="header-search" href="/#catalog"><SearchIcon /><span>商品を検索</span></Link>
      <div className="header-actions"><Link className="account-link" href="/#about"><UserIcon /><span>ログイン</span></Link><button className="cart-button" onClick={openCart} aria-label={`カート、${count}点`}><BagIcon /><span>カート</span><b>{count}</b></button></div>
    </header>
    <nav className="commerce-nav shell" aria-label="商品カテゴリー"><Link href="/#catalog">すべての商品</Link><Link href="/#catalog">イラスト</Link><Link href="/#catalog">テンプレート</Link><Link href="/#catalog">音楽・サウンド</Link><Link href="/#catalog">便利ツール</Link><Link className="creator-link" href="/#about">作品を販売する →</Link></nav>
  </>;
}
