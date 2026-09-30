import Link from "next/link";
import { notFound } from "next/navigation";
import { connection } from "next/server";
import { EmptyState } from "@/components/common/empty-state";
import { AddToCartButton } from "@/components/products/add-to-cart-button";
import { ProductVisual } from "@/components/products/product-visual";
import { getProduct } from "@/lib/api";
import { categoryName } from "@/lib/categories";
import { formatPrice } from "@/lib/format";
import { productsHref } from "@/lib/products";

export default async function ProductPage({ params }: PageProps<"/products/[id]">) {
  await connection();
  const id = Number((await params).id);
  if (!Number.isInteger(id) || id < 1) notFound();

  let product;
  try {
    product = await getProduct(id);
  } catch {
    return (
      <div className="shell">
        <EmptyState title="商品情報を取得できませんでした" body="pingu-apiの起動状態を確認して、もう一度お試しください。" href="/products" action="商品一覧へ戻る" />
      </div>
    );
  }
  if (!product) notFound();

  return (
    <div className="shell product-page">
      <nav className="breadcrumb" aria-label="パンくず">
        <Link href="/products">商品を探す</Link><span aria-hidden="true">/</span>
        <Link href={productsHref({ category: product.categoryId })}>{categoryName(product.categoryId)}</Link>
      </nav>
      <div className="product-detail">
        <div className="detail-visual"><ProductVisual product={product} /></div>
        <div className="detail-copy">
          <p className="detail-category">{categoryName(product.categoryId)}</p>
          <h1>{product.name}</h1>
          <p className="detail-price">{formatPrice(product.price)}</p>
          <p className="detail-description">{product.description || "つくり手のこだわりが詰まったデジタルプロダクトです。"}</p>
          <AddToCartButton product={product} />
          <dl className="detail-facts">
            <div><dt>形式</dt><dd>デジタルダウンロード</dd></div>
            <div><dt>受け取り</dt><dd>購入後すぐに利用可能</dd></div>
            <div><dt>状態</dt><dd>{product.status || "available"}</dd></div>
          </dl>
        </div>
      </div>
    </div>
  );
}
