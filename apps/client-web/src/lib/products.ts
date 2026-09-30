import type { Product } from "./types";

export const PAGE_SIZE = 8;

export const sortOptions = [
  { value: "newest", label: "新着順" },
  { value: "price-low", label: "価格の安い順" },
  { value: "price-high", label: "価格の高い順" },
] as const;
export type SortValue = (typeof sortOptions)[number]["value"];

export type ProductQuery = { q: string; category: number | null; sort: SortValue; page: number };
type RawParams = Record<string, string | string[] | undefined>;

const first = (value: string | string[] | undefined) => (Array.isArray(value) ? value[0] : value) ?? "";

export function parseProductQuery(params: RawParams): ProductQuery {
  const category = Number(first(params.category));
  const page = Number(first(params.page));
  const sort = first(params.sort);
  return {
    q: first(params.q).trim(),
    category: Number.isInteger(category) && category > 0 ? category : null,
    sort: sortOptions.some((option) => option.value === sort) ? (sort as SortValue) : "newest",
    page: Number.isInteger(page) && page > 0 ? page : 1,
  };
}

export function productsHref(query: Partial<ProductQuery>) {
  const search = new URLSearchParams();
  if (query.q) search.set("q", query.q);
  if (query.category) search.set("category", String(query.category));
  if (query.sort && query.sort !== "newest") search.set("sort", query.sort);
  if (query.page && query.page > 1) search.set("page", String(query.page));
  const text = search.toString();
  return text ? `/products?${text}` : "/products";
}

export function queryProducts(products: Product[], query: ProductQuery) {
  const needle = query.q.toLocaleLowerCase("ja");
  const filtered = products.filter((product) =>
    (query.category === null || product.categoryId === query.category) &&
    (!needle || `${product.name} ${product.description}`.toLocaleLowerCase("ja").includes(needle)));
  const sorted = [...filtered].sort((a, b) =>
    query.sort === "price-low" ? a.price - b.price : query.sort === "price-high" ? b.price - a.price : b.id - a.id);
  const pageCount = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE));
  const page = Math.min(query.page, pageCount);
  return { total: sorted.length, page, pageCount, items: sorted.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE) };
}
