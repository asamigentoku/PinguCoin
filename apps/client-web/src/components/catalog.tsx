"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import type { Product } from "@/lib/types";
import { useCart } from "./cart-provider";
import { ArrowIcon, HeartIcon, SearchIcon } from "./icons";

const categoryNames: Record<number, string> = { 1: "アート", 2: "テンプレート", 3: "音楽", 4: "ツール" };
const pageSize = 8;

export function Catalog({ products, apiAvailable }: { products: Product[]; apiAvailable: boolean }) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<number | "all">("all");
  const [sort, setSort] = useState("featured");
  const [currentPage, setCurrentPage] = useState(1);
  const [favorites, setFavorites] = useState<number[]>([]);
  const { addItem } = useCart();
  const categories = useMemo(() => [...new Set(products.map((product) => product.categoryId))], [products]);
  const visibleProducts = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase("ja");
    const filtered = products.filter((product) => (category === "all" || product.categoryId === category) && (!normalizedQuery || `${product.name} ${product.description}`.toLocaleLowerCase("ja").includes(normalizedQuery)));
    return [...filtered].sort((a, b) => sort === "price-low" ? a.price - b.price : sort === "price-high" ? b.price - a.price : sort === "newest" ? b.id - a.id : 0);
  }, [category, products, query, sort]);
  const pageCount = Math.max(1, Math.ceil(visibleProducts.length / pageSize));
  const activePage = Math.min(currentPage, pageCount);
  const pageProducts = visibleProducts.slice((activePage - 1) * pageSize, activePage * pageSize);
  const goToPage = (page: number) => {
    setCurrentPage(Math.max(1, Math.min(page, pageCount)));
    document.querySelector("#catalog")?.scrollIntoView({ behavior: "smooth" });
  };

  return (
    <section className="catalog shell" id="catalog" aria-labelledby="catalog-title">
      <div className="section-heading"><div><p className="eyebrow">EXPLORE THE MARKET</p><h2 id="catalog-title">商品を探す</h2></div><p>{String(visibleProducts.length).padStart(2, "0")} PRODUCTS</p></div>
      <div className="catalog-tools">
        <div className="category-tabs" role="group" aria-label="カテゴリー"><button className={category === "all" ? "active" : ""} onClick={() => { setCategory("all"); setCurrentPage(1); }}>すべて</button>{categories.map((id) => <button key={id} className={category === id ? "active" : ""} onClick={() => { setCategory(id); setCurrentPage(1); }}>{categoryNames[id] ?? `カテゴリー ${id}`}</button>)}</div>
        <div className="catalog-controls"><label className="search-field"><SearchIcon /><span className="sr-only">商品を検索</span><input value={query} onChange={(event) => { setQuery(event.target.value); setCurrentPage(1); }} placeholder="作品を検索" /></label><label className="sort-field"><span className="sr-only">並び順</span><select value={sort} onChange={(event) => { setSort(event.target.value); setCurrentPage(1); }}><option value="featured">おすすめ順</option><option value="newest">新着順</option><option value="price-low">価格の安い順</option><option value="price-high">価格の高い順</option></select></label></div>
      </div>
      {!apiAvailable ? <CatalogState mark="!" title="マーケットに接続できませんでした" body="pingu-apiを起動すると、ここに商品が表示されます。" /> : visibleProducts.length === 0 ? <CatalogState mark="○" title={products.length ? "条件に合う作品がありません" : "最初の作品を待っています"} body={products.length ? "検索条件を変えてみてください。" : "商品が登録されると、このマーケットに並びます。"} /> : (
        <><div className="result-summary"><span>{visibleProducts.length}件中 {(activePage - 1) * pageSize + 1}–{Math.min(activePage * pageSize, visibleProducts.length)}件を表示</span><span>{activePage} / {pageCount} ページ</span></div>
        <div className="product-grid">{pageProducts.map((product, index) => <article className="product-card" key={product.id}>
          <div className="product-image-wrap"><Link href={`/products/${product.id}`} className="product-image-link" aria-label={`${product.name}の詳細を見る`}><ProductVisual product={product} index={index} /></Link><button className={`favorite-button ${favorites.includes(product.id) ? "active" : ""}`} onClick={() => setFavorites((current) => current.includes(product.id) ? current.filter((id) => id !== product.id) : [...current, product.id])} aria-label={`${product.name}をお気に入り${favorites.includes(product.id) ? "から削除" : "に追加"}`}><HeartIcon /></button></div>
          <div className="product-seller">PINGU CREATOR #{String(product.userId).padStart(3, "0")} <span>★ 4.{7 + product.id % 3}</span></div>
          <div className="product-meta"><div><p>{categoryNames[product.categoryId] ?? `CATEGORY ${product.categoryId}`}</p><Link href={`/products/${product.id}`}>{product.name}</Link></div><div className="product-price"><strong>{formatPrice(product.price)}</strong><small>税込</small></div></div>
          <button className="quick-add" onClick={() => addItem(product)}>カートに追加 <ArrowIcon size={17} /></button>
        </article>)}</div>
        {pageCount > 1 && <nav className="pagination" aria-label="商品一覧のページ"><button onClick={() => goToPage(activePage - 1)} disabled={activePage === 1}>← 前へ</button><div>{Array.from({ length: pageCount }, (_, index) => index + 1).map((page) => <button key={page} className={page === activePage ? "active" : ""} onClick={() => goToPage(page)} aria-current={page === activePage ? "page" : undefined}>{page}</button>)}</div><button onClick={() => goToPage(activePage + 1)} disabled={activePage === pageCount}>次へ →</button></nav>}</>
      )}
    </section>
  );
}

function CatalogState({ mark, title, body }: { mark: string; title: string; body: string }) { return <div className="catalog-state"><span className="state-mark">{mark}</span><div><h3>{title}</h3><p>{body}</p></div></div>; }

export function ProductVisual({ product, index = 0 }: { product: Product; index?: number }) {
  return <div className={`product-visual visual-${index % 4}`} style={product.imageUrl ? { backgroundImage: `url(${product.imageUrl})` } : undefined}>{!product.imageUrl && <span>{product.name.slice(0, 1).toUpperCase()}</span>}<small>{product.status || "AVAILABLE"}</small></div>;
}

export function formatPrice(value: number) { return new Intl.NumberFormat("ja-JP", { style: "currency", currency: "JPY", maximumFractionDigits: 0 }).format(value); }
