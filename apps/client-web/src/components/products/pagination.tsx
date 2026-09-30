import Link from "next/link";
import { productsHref, type ProductQuery } from "@/lib/products";

export function Pagination({ query, page, pageCount }: { query: ProductQuery; page: number; pageCount: number }) {
  if (pageCount <= 1) return null;
  const href = (target: number) => productsHref({ ...query, page: target });
  return (
    <nav className="pagination" aria-label="ページ送り">
      {page > 1 ? <Link href={href(page - 1)} rel="prev">前へ</Link> : <span className="is-disabled">前へ</span>}
      <div>
        {Array.from({ length: pageCount }, (_, index) => index + 1).map((n) => (
          <Link key={n} href={href(n)} className={n === page ? "is-active" : undefined} aria-current={n === page ? "page" : undefined} aria-label={`${n}ページ目`}>{n}</Link>
        ))}
      </div>
      {page < pageCount ? <Link href={href(page + 1)} rel="next">次へ</Link> : <span className="is-disabled">次へ</span>}
    </nav>
  );
}
