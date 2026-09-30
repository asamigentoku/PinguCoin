import type { Metadata } from "next";
import { PageHeader } from "@/components/common/page-header";
import { Newsletter } from "@/components/home/newsletter";

export const metadata: Metadata = { title: "PinguCoinについて" };

const points = [
  { title: "厳選された作品", body: "公開前に内容を確認し、安心して使える作品だけを並べています。" },
  { title: "シンプルな購入体験", body: "探して、買って、すぐダウンロード。迷わない導線を大切にしています。" },
  { title: "つくり手へ直接還元", body: "売上はクリエイターに届き、次の作品づくりを支えます。" },
];

export default function AboutPage() {
  return (
    <>
      <PageHeader title="PinguCoinについて" lead="個人クリエイターと、新しい表現を探す人をつなぐマーケットです。" />
      <section className="shell prose-section">
        <p className="prose-lead">小さなアイデアが、誰かの大切なひとつになる。その瞬間を支えるために、PinguCoinは生まれました。</p>
        <div className="point-list">
          {points.map((point) => <div key={point.title}><h2>{point.title}</h2><p>{point.body}</p></div>)}
        </div>
      </section>
      <Newsletter />
    </>
  );
}
