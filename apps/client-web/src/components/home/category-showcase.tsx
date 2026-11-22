import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { categories } from "@/lib/categories";
import { productsHref } from "@/lib/products";

const categoryKeys = ["art", "templates", "music", "tools"] as const;

export function CategoryShowcase() {
  const homeT = useTranslations("Home");
  const productT = useTranslations("Products");
  return (
    <section className="home-section shell" aria-labelledby="category-title">
      <div className="section-heading"><h2 id="category-title">{homeT("categories")}</h2><Link className="text-link" href="/products">{homeT("allProducts")}</Link></div>
      <div className="category-grid">
        {categories.map((category, index) => (
          <Link key={category.id} href={productsHref({ category: category.id })} className={`category-card ${category.tone}`}>
            <strong>{productT(categoryKeys[index])}</strong>
          </Link>
        ))}
      </div>
    </section>
  );
}
