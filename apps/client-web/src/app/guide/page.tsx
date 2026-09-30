import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader } from "@/components/common/page-header";

export const metadata: Metadata = { title: "ご利用ガイド" };

const steps = [
  { title: "作品を探す", body: "カテゴリーや検索から、気になる作品を見つけます。" },
  { title: "カートに入れる", body: "詳細ページか一覧から、カートに追加します。" },
  { title: "購入してダウンロード", body: "決済が完了すると、すぐにダウンロードできます。" },
];
const faqs = [
  { q: "購入した作品はいつ使えますか？", a: "決済の完了後、すぐにダウンロードして使えます。" },
  { q: "決済方法は何がありますか？", a: "現在、購入機能を準備中です。公開時にこのページでお知らせします。" },
  { q: "作品を販売するには？", a: "クリエイター向けの受付を準備中です。お問い合わせフォームからご連絡ください。" },
];

export default function GuidePage() {
  return (
    <>
      <PageHeader title="ご利用ガイド" lead="はじめての方でも、3つのステップで購入できます。" />
      <section className="shell prose-section">
        <ol className="step-list">
          {steps.map((step) => <li key={step.title}><h2>{step.title}</h2><p>{step.body}</p></li>)}
        </ol>
        <h2 className="prose-title">よくある質問</h2>
        <div className="faq-list">
          {faqs.map((faq) => <details key={faq.q}><summary>{faq.q}</summary><p>{faq.a}</p></details>)}
        </div>
        <p className="prose-cta">解決しない場合は、<Link className="text-link" href="/contact">お問い合わせ</Link>ください。</p>
      </section>
    </>
  );
}
