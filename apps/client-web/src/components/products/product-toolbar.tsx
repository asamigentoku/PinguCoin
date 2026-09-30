import Link from "next/link";
import { categories } from "@/lib/categories";
import { productsHref, sortOptions, type ProductQuery } from "@/lib/products";
import { SearchIcon } from "../ui/icons";

export function ProductToolbar({ query }: { query: ProductQuery }) {
  return (
    <div className="product-toolbar">
      <div className="category-tabs" role="group" aria-label="カテゴリー">
        <Link href={productsHref({ ...query, category: null, page: 1 })} className={query.category === null ? "is-active" : undefined}>すべて</Link>
        {categories.map((category) => (
          <Link key={category.id} href={productsHref({ ...query, category: category.id, page: 1 })} className={query.category === category.id ? "is-active" : undefined}>{category.name}</Link>
        ))}
      </div>
      <form className="toolbar-form" action="/products" method="get">
        {query.category && <input type="hidden" name="category" value={query.category} />}
        <label className="search-field"><SearchIcon size={18} /><span className="sr-only">商品を検索</span><input name="q" defaultValue={query.q} placeholder="作品を検索" /></label>
        <label className="sort-field"><span className="sr-only">並び順</span>
          <select name="sort" defaultValue={query.sort}>{sortOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
        </label>
        <button className="button button-primary toolbar-submit" type="submit">絞り込む</button>
      </form>
    </div>
  );
}
