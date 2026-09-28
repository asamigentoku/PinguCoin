import { connection } from "next/server";
import { Catalog } from "@/components/catalog";
import { NewsletterForm } from "@/components/newsletter-form";
import { getProducts } from "@/lib/api";
import type { Product } from "@/lib/types";

export default async function Home() {
  await connection();
  let products: Product[] = [];
  let apiAvailable = true;

  try {
    products = await getProducts();
  } catch {
    apiAvailable = false;
  }

  return (
    <main>
      <section className="hero shell" aria-labelledby="hero-title">
        <div className="hero-copy">
          <p className="eyebrow">A SMALL MARKET FOR BIG IDEAS</p>
          <h1 id="hero-title">まだ知らない作品と、<span>出会う場所。</span></h1>
          <p className="hero-lead">つくり手の個性が宿るデジタルプロダクトを、見つけて、集めて、楽しもう。</p>
          <div className="hero-actions">
            <a className="button button-primary" href="#catalog">商品を見つける <span aria-hidden="true">↘</span></a>
            <a className="text-link" href="#about">PinguCoinについて <span aria-hidden="true">→</span></a>
          </div>
          <div className="hero-proof"><span><b>NEW</b> 毎週、新着作品を追加</span><span><b>FAST</b> 購入後すぐにダウンロード</span></div>
        </div>
        <div className="hero-art" aria-label="PinguCoinのマーケットを表現したイラスト">
          <div className="orbit orbit-one" /><div className="orbit orbit-two" />
          <div className="coin coin-one">P</div><div className="coin coin-two">¥</div>
          <div className="penguin"><div className="penguin-face"><span className="eye eye-left" /><span className="eye eye-right" /><span className="beak" /></div></div>
          <p className="art-note">CURATED WITH CARE<br />IN TOKYO</p>
        </div>
      </section>

      <section className="ticker" aria-label="サービスの特徴"><div>
        <span>ORIGINAL WORKS</span><i>✦</i><span>SECURE CHECKOUT</span><i>✦</i><span>INSTANT DOWNLOAD</span><i>✦</i><span>CREATOR FIRST</span><i>✦</i><span>ORIGINAL WORKS</span><i>✦</i>
      </div></section>

      <section className="category-showcase shell" aria-labelledby="category-title">
        <div className="category-heading"><div><p className="eyebrow">SHOP BY CATEGORY</p><h2 id="category-title">カテゴリーから探す</h2></div><a href="#catalog">すべて見る →</a></div>
        <div className="category-grid">
          <a href="#catalog" className="category-card category-art"><span>01</span><div><b>ART & ILLUSTRATION</b><strong>アート・イラスト</strong></div><i>→</i></a>
          <a href="#catalog" className="category-card category-template"><span>02</span><div><b>DESIGN TEMPLATE</b><strong>テンプレート</strong></div><i>→</i></a>
          <a href="#catalog" className="category-card category-sound"><span>03</span><div><b>MUSIC & SOUND</b><strong>音楽・サウンド</strong></div><i>→</i></a>
          <a href="#catalog" className="category-card category-tool"><span>04</span><div><b>TOOLS</b><strong>便利ツール</strong></div><i>→</i></a>
        </div>
      </section>

      <section className="feature-drop shell" aria-label="今週の特集">
        <div className="feature-drop-art"><span>NEW<br />DROP</span><i>✦</i><i>✦</i><i>✦</i></div>
        <div className="feature-drop-copy"><p className="eyebrow">WEEKLY EDIT · VOL. 09</p><h2>暮らしを少し変える、<br />小さなデジタル道具。</h2><p>今週は、毎日の制作や仕事を心地よく整えるテンプレートとツールを集めました。</p><a className="button feature-button" href="#catalog">特集を見る <span>→</span></a></div>
        <div className="feature-drop-label"><span>CURATED</span><strong>09</strong></div>
      </section>

      <Catalog products={products} apiAvailable={apiAvailable} />

      <section className="shopping-benefits shell" aria-label="購入サービス">
        <div><span>01</span><strong>すぐに受け取れる</strong><p>決済完了後、デジタル作品をすぐにダウンロード。</p></div>
        <div><span>02</span><strong>安心して購入</strong><p>安全な決済と購入履歴で、大切な作品を管理。</p></div>
        <div><span>03</span><strong>つくり手を応援</strong><p>購入代金は作品を生み出したクリエイターへ。</p></div>
      </section>

      <section className="about shell" id="about">
        <div className="about-number">01</div>
        <div className="about-heading"><p className="eyebrow">WHY PINGUCOIN?</p><h2>好きの価値を、<br />もっと近くに。</h2></div>
        <div className="about-copy">
          <p>PinguCoinは、個人クリエイターと新しい表現を探す人をつなぐマーケットです。小さなアイデアが、誰かの大切なひとつになる。その瞬間を支えます。</p>
          <div className="about-points"><span><b>01</b> 厳選された作品</span><span><b>02</b> シンプルな購入体験</span><span><b>03</b> つくり手へ直接還元</span></div>
        </div>
      </section>

      <section className="newsletter shell">
        <div><p className="eyebrow">PINGU LETTER</p><h2>新しい作品を、<br />週末の受信箱へ。</h2></div>
        <NewsletterForm />
      </section>
    </main>
  );
}
