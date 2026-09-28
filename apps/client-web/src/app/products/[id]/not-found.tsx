import Link from "next/link";
export default function ProductNotFound() { return <main className="detail-state shell"><p className="eyebrow">404 · NOT FOUND</p><h1>この商品は見つかりませんでした。</h1><p>販売が終了したか、URLが変更された可能性があります。</p><Link className="button button-primary" href="/">マーケットへ戻る</Link></main>; }
