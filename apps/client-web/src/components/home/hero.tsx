import Link from "next/link";

export function Hero() {
  return (
    <section className="hero shell" aria-labelledby="hero-title">
      <div className="hero-copy">
        <h1 id="hero-title">好きの価値を、<br />もっと近くに。</h1>
        <p>個人クリエイターのイラスト、テンプレート、音楽、ツール。買ってすぐ使えるデジタル作品のマーケットです。</p>
        <div className="hero-actions">
          <Link className="button button-coin" href="/products">商品を探す</Link>
          <Link className="text-link" href="/about">PinguCoinについて</Link>
        </div>
      </div>
      <div className="hero-art" aria-hidden="true">
        <div className="hero-coin">
          <div className="penguin"><span className="eye eye-left" /><span className="eye eye-right" /><span className="beak" /></div>
        </div>
        <span className="float-coin float-one">¥</span>
        <span className="float-coin float-two">P</span>
        <span className="float-tag">購入後すぐ<br />ダウンロード</span>
      </div>
    </section>
  );
}
