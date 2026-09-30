import type { Metadata } from "next";
import Link from "next/link";
import { auth } from "@clerk/nextjs/server";
import { EmptyState } from "@/components/common/empty-state";
import { PageHeader } from "@/components/common/page-header";
import { SignInRequired } from "@/components/sell/sign-in-required";
import { formatPoints } from "@/lib/points";
import { getPoints, type PointSummary } from "@/lib/shop";

export const metadata: Metadata = { title: "ポイントをためる" };

const typeLabel: Record<string, string> = { credit: "付与", debit: "消費", payment: "購入", refund: "返金" };

export default async function PointsPage() {
  const { userId } = await auth();
  if (!userId) return <SignInRequired title="ポイントをためる" />;

  let points: PointSummary | null = null;
  try {
    points = await getPoints();
  } catch {
    points = null;
  }

  return (
    <>
      <PageHeader title="ポイントをためる" lead="ポイントは、作品の購入に使えます。" />
      <section className="shell sell-section">
        {!points ? (
          <EmptyState title="ポイントを読み込めませんでした" body="pingu-apiとpayment-apiが起動しているか確認して、ページを再読み込みしてください。" href="/points" action="再読み込みする" />
        ) : (
          <>
            <div className="points-hero">
              <span className="point-balance-coin" aria-hidden="true">P</span>
              <div><small>保有ポイント</small><strong>{formatPoints(points.balance)}<span> pt</span></strong></div>
              <Link className="button button-coin" href="/products">作品を探す</Link>
            </div>
            <h2 className="points-heading">ポイントの履歴</h2>
            <ul className="points-history">
              {points.transactions.map((transaction) => (
                <li key={transaction.id}>
                  <span className="status-badge is-draft">{typeLabel[transaction.type] ?? transaction.type}</span>
                  <span className="points-reason">{transaction.reason}</span>
                  <time dateTime={transaction.created_at}>{new Date(transaction.created_at).toLocaleString("ja-JP")}</time>
                  <strong className={transaction.amount < 0 ? "is-minus" : "is-plus"}>{transaction.amount > 0 && "+"}{formatPoints(transaction.amount)} pt</strong>
                </li>
              ))}
            </ul>
            <p className="points-note">初めての方には、ウェルカムボーナスとして {formatPoints(1600)} pt を1回だけ付与しています。ほかのためかた(出品の売上など)は準備中です。</p>
          </>
        )}
      </section>
    </>
  );
}
