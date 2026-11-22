import { useLocale, useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { categories } from "@/lib/categories";
import { productsHref, sortOptions, type ProductQuery } from "@/lib/products";
import { SearchIcon } from "../ui/icons";

const categoryKeys = ["art", "templates", "music", "tools"] as const;

export function ProductToolbar({ query }: { query: ProductQuery }) {
  const locale = useLocale();
  const t = useTranslations("Products");
  return (
    <div className="product-toolbar">
      <div className="category-tabs" role="group" aria-label="Category">
        <Link href={productsHref({ ...query, category: null, page: 1 })} className={query.category === null ? "is-active" : undefined}>{t("all")}</Link>
        {categories.map((category, index) => (
          <Link key={category.id} href={productsHref({ ...query, category: category.id, page: 1 })} className={query.category === category.id ? "is-active" : undefined}>{t(categoryKeys[index])}</Link>
        ))}
      </div>
      <form className="toolbar-form" action={`/${locale}/products`} method="get">
        {query.category && <input type="hidden" name="category" value={query.category} />}
        <label className="search-field"><SearchIcon size={18} /><span className="sr-only">{t("search")}</span><input name="q" defaultValue={query.q} placeholder={t("search")} /></label>
        <label className="sort-field"><span className="sr-only">{t("sortLabel")}</span>
          <select name="sort" defaultValue={query.sort}>{sortOptions.map((option) => <option key={option.value} value={option.value}>{t(({ newest: "newest", "price-low": "priceLow", "price-high": "priceHigh" } as const)[option.value])}</option>)}</select>
        </label>
        <button className="button button-primary toolbar-submit" type="submit">{t("filter")}</button>
      </form>
    </div>
  );
}
