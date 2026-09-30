"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { deleteListing, setListingStatus } from "@/app/sell/actions";
import { categoryName } from "@/lib/categories";
import { formatPrice } from "@/lib/format";
import { statusLabel } from "@/lib/listing";
import type { Product } from "@/lib/types";
import { ProductVisual } from "../products/product-visual";

export function ManageList({ products, fileCounts }: { products: Product[]; fileCounts: Record<number, number> }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState("");

  function run(task: () => Promise<{ ok: boolean; error?: string }>) {
    setError("");
    startTransition(async () => {
      const result = await task();
      if (!result.ok) setError(result.error ?? "処理に失敗しました。");
      router.refresh();
    });
  }

  return (
    <>
      {error && <p className="form-error" role="alert">{error}</p>}
      <ul className="manage-list">
        {products.map((product) => {
          const draft = product.status === "draft";
          return (
            <li key={product.id} className="manage-item">
              <div className="manage-thumb"><ProductVisual product={product} /></div>
              <div className="manage-info">
                <span className={`status-badge ${draft ? "is-draft" : "is-live"}`}>{statusLabel(product.status)}</span>
                <h2>{product.name}</h2>
                <p>{categoryName(product.categoryId)} ・ {formatPrice(product.price)}</p>
                <small>{product.imageUrl ? "画像あり" : "画像なし"} / {fileCounts[product.id] ? `ファイル${fileCounts[product.id]}個` : "ファイルなし"}</small>
              </div>
              <div className="manage-actions">
                {!draft && <Link href={`/products/${product.id}`}>ページを見る</Link>}
                <Link href={`/sell/manage/${product.id}`}>編集</Link>
                <button type="button" disabled={pending} onClick={() => run(() => setListingStatus(product.id, draft ? "available" : "draft"))}>{draft ? "公開する" : "非公開にする"}</button>
                <button type="button" className="is-danger" disabled={pending} onClick={() => confirm(`「${product.name}」を削除しますか？ 画像とファイルも削除され、元に戻せません。`) && run(() => deleteListing(product.id))}>削除</button>
              </div>
            </li>
          );
        })}
      </ul>
    </>
  );
}
