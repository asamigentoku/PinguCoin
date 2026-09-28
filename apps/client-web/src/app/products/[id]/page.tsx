import Link from "next/link";
import { connection } from "next/server";
import { notFound } from "next/navigation";
import { ProductActions } from "@/components/product-actions";
import { ProductVisual, formatPrice } from "@/components/catalog";
import { getProduct } from "@/lib/api";

export default async function ProductPage({ params }: PageProps<"/products/[id]">) {
  await connection();
  const { id: rawId } = await params;
  const id = Number(rawId);
  if (!Number.isInteger(id) || id < 1) notFound();
  let product;
  try { product = await getProduct(id); } catch {
    return <main className="detail-state shell"><p className="eyebrow">CONNECTION ERROR</p><h1>商品情報を取得できませんでした。</h1><p>pingu-apiの起動状態を確認して、もう一度お試しください。</p><Link className="button button-primary" href="/">マーケットへ戻る</Link></main>;
  }
  if (!product) notFound();
  return <main className="product-page shell">
    <div className="product-breadcrumb"><Link href="/">MARKET</Link><span>/</span><span>PRODUCT {String(product.id).padStart(3, "0")}</span></div>
    <div className="product-detail"><div className="detail-visual"><ProductVisual product={product} index={product.id} /></div><div className="detail-copy">
      <p className="eyebrow">DIGITAL PRODUCT · #{String(product.id).padStart(3, "0")}</p><h1>{product.name}</h1><p className="detail-price">{formatPrice(product.price)}</p>
      <p className="detail-description">{product.description || "つくり手のこだわりが詰まったデジタルプロダクトです。"}</p><ProductActions product={product} />
      <dl className="detail-facts"><div><dt>FORMAT</dt><dd>デジタルダウンロード</dd></div><div><dt>DELIVERY</dt><dd>購入後すぐに利用可能</dd></div><div><dt>STATUS</dt><dd>{product.status || "available"}</dd></div></dl>
    </div></div>
  </main>;
}
