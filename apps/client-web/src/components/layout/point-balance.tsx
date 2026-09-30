import Link from "next/link";
import { auth } from "@clerk/nextjs/server";
import { formatPoints } from "@/lib/points";
import { getPoints } from "@/lib/shop";

// ログイン中のユーザーの、実際のポイント残高。取得できないとき(未ログイン・API停止)は表示しない。
export async function PointBalance() {
  const { userId } = await auth();
  if (!userId) return null;
  const points = await getPoints().catch(() => null);
  if (!points) return null;
  return (
    <Link className="point-balance" href="/points" aria-label={`保有ポイント ${formatPoints(points.balance)}ポイント`}>
      <span className="point-balance-coin" aria-hidden="true">P</span>
      <span>{formatPoints(points.balance)}<small> pt</small></span>
    </Link>
  );
}
