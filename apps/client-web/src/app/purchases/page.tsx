import type { Metadata } from "next";
import Link from "next/link";
import { auth } from "@clerk/nextjs/server";
import { EmptyState } from "@/components/common/empty-state";
import { PageHeader } from "@/components/common/page-header";
import { ProductVisual } from "@/components/products/product-visual";
import { SignInRequired } from "@/components/sell/sign-in-required";
import { DownloadButton } from "@/components/shop/download-button";
import { getProduct } from "@/lib/api";
import { formatPrice } from "@/lib/format";
import { formatBytes } from "@/lib/listing";
import { getProductFiles, type ProductFile } from "@/lib/seller";
import { getOrders, type Order } from "@/lib/shop";
import type { Product } from "@/lib/types";

export const metadata: Metadata = { title: "購入履歴" };

const statusText: Record<string, string> = { paid: "購入済み", pending: "処理中", failed: "失敗", canceled: "キャンセル" };

export default async function PurchasesPage() {
  const { userId } = await auth();
  if (!userId) return <SignInRequired title="購入履歴" />;

  let orders: Order[] = [];
  let apiAvailable = true;
  try {
    orders = (await getOrders()).sort((a, b) => b.id - a.id);
  } catch {
    apiAvailable = false;
  }
  const products = new Map<number, Product | null>(
    await Promise.all(orders.map(async (order) => [order.product_id, await getProduct(order.product_id).catch(() => null)] as const)),
  );

  const files = new Map<number, ProductFile[]>(
    await Promise.all(orders.map(async (order) => [order.product_id, await getProductFiles(order.product_id).catch(() => [])] as const)),
  );

  return (
    <>
      <PageHeader title="購入履歴" lead="購入した作品は、ここからいつでもダウンロードできます。" />
      <section className="shell sell-section">
        {!apiAvailable ? (
          <EmptyState title="購入履歴を読み込めませんでした" body="pingu-apiが起動しているか確認して、ページを再読み込みしてください。" href="/purchases" action="再読み込みする" />
        ) : orders.length === 0 ? (
          <EmptyState title="購入した作品はまだありません" body="気になる作品を探してみましょう。" href="/products" action="商品を探す" />
        ) : (
          <ul className="manage-list">
            {orders.map((order) => {
              const product = products.get(order.product_id);
              return (
                <li key={order.id} className="manage-item">
                  <div className="manage-thumb">{product ? <ProductVisual product={product} /> : <div className="product-visual" />}</div>
                  <div className="manage-info">
                    <span className={`status-badge ${order.status === "paid" ? "is-live" : "is-draft"}`}>{statusText[order.status] ?? order.status}</span>
                    <h2>{product ? <Link href={`/products/${product.id}`}>{product.name}</Link> : "削除された商品"}</h2>
                    <p>{formatPrice(order.total_amount)}{order.quantity > 1 && ` (${order.quantity}点)`} ・ 注文番号 {order.id}</p>
                  </div>
                  {order.status === "paid" && (files.get(order.product_id) ?? []).length > 0 && (
                    <ul className="download-list" aria-label="ダウンロード">
                      {(files.get(order.product_id) ?? []).map((file) => (
                        <li key={file.id}><span>{file.originalFilename}<small>{formatBytes(file.fileSize)}</small></span><DownloadButton assetId={file.id} /></li>
                      ))}
                    </ul>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </>
  );
}
