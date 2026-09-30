const benefits = [
  { title: "すぐに受け取れる", body: "決済が終わったらそのままダウンロード。待ち時間はありません。" },
  { title: "安心して購入できる", body: "安全な決済と購入履歴で、買った作品をあとから見返せます。" },
  { title: "つくり手に届く", body: "購入代金は作品を生み出したクリエイターへ直接還元されます。" },
];

export function Benefits() {
  return (
    <section className="home-section shell" aria-label="PinguCoinの特長">
      <div className="benefits">{benefits.map((b) => <div key={b.title}><strong>{b.title}</strong><p>{b.body}</p></div>)}</div>
    </section>
  );
}
