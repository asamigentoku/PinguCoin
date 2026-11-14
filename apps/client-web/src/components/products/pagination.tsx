import { Link } from "@/i18n/navigation";
import { useTranslations } from "next-intl";
import { productsHref, type ProductQuery } from "@/lib/products";

export function Pagination({ query, page, pageCount }: { query: ProductQuery; page: number; pageCount: number }) {
  const t = useTranslations("Products");
  if (pageCount <= 1) return null;
  const href = (target: number) => productsHref({ ...query, page: target });
  return (
    <nav className="pagination" aria-label={t("paginationLabel")}>
      {page > 1 ? <Link href={href(page - 1)} rel="prev">{t("previous")}</Link> : <span className="is-disabled">{t("previous")}</span>}
      <div>
        {Array.from({ length: pageCount }, (_, index) => index + 1).map((n) => (
          <Link key={n} href={href(n)} className={n === page ? "is-active" : undefined} aria-current={n === page ? "page" : undefined} aria-label={t("pageLabel", { page: n })}>{n}</Link>
        ))}
      </div>
      {page < pageCount ? <Link href={href(page + 1)} rel="next">{t("next")}</Link> : <span className="is-disabled">{t("next")}</span>}
    </nav>
  );
}
