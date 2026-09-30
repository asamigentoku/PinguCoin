"use client";

import { useState } from "react";
import { categoryNames, formatDate, formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";

const pageSize = 10;

export function ProductsTable({ products }: { products: Product[] }) {
  const [page, setPage] = useState(1);
  const pageCount = Math.max(1, Math.ceil(products.length / pageSize));
  const activePage = Math.min(page, pageCount);
  const pageProducts = products.slice((activePage - 1) * pageSize, activePage * pageSize);
  const goToPage = (next: number) => {
    setPage(Math.max(1, Math.min(next, pageCount)));
    document.querySelector("#admin-table-top")?.scrollIntoView({ behavior: "smooth" });
  };

  return (
    <div id="admin-table-top">
      <div className="result-summary">
        <span>{products.length}件中 {(activePage - 1) * pageSize + 1}–{Math.min(activePage * pageSize, products.length)}件を表示</span>
        <span>{activePage} / {pageCount} ページ</span>
      </div>
      <div className="admin-table-scroll">
        <table className="admin-table">
          <thead>
            <tr><th>ID</th><th>商品名</th><th>カテゴリー</th><th>出品者</th><th className="is-numeric">価格</th><th>ステータス</th><th>更新日</th></tr>
          </thead>
          <tbody>
            {pageProducts.map((product) => (
              <tr key={product.id}>
                <td>#{String(product.id).padStart(3, "0")}</td>
                <td>{product.name}</td>
                <td>{categoryNames[product.categoryId] ?? `カテゴリー ${product.categoryId}`}</td>
                <td>PINGU CREATOR #{String(product.userId).padStart(3, "0")}</td>
                <td className="is-numeric">{formatPrice(product.price)}</td>
                <td><span className={`status-pill ${product.status === "AVAILABLE" ? "" : "is-muted"}`}>{product.status || "AVAILABLE"}</span></td>
                <td>{formatDate(product.updatedAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pageCount > 1 && (
        <nav className="pagination" aria-label="商品一覧のページ">
          <button onClick={() => goToPage(activePage - 1)} disabled={activePage === 1}>← 前へ</button>
          <div>
            {Array.from({ length: pageCount }, (_, index) => index + 1).map((p) => (
              <button key={p} className={p === activePage ? "active" : ""} onClick={() => goToPage(p)} aria-current={p === activePage ? "page" : undefined}>{p}</button>
            ))}
          </div>
          <button onClick={() => goToPage(activePage + 1)} disabled={activePage === pageCount}>次へ →</button>
        </nav>
      )}
    </div>
  );
}
