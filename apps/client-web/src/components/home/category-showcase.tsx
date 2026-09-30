import Link from "next/link";
import { categories } from "@/lib/categories";
import { productsHref } from "@/lib/products";

export function CategoryShowcase() {
  return (
    <section className="home-section shell" aria-labelledby="category-title">
      <div className="section-heading"><h2 id="category-title">カテゴリーから探す</h2><Link className="text-link" href="/products">すべての商品</Link></div>
      <div className="category-grid">
        {categories.map((category) => (
          <Link key={category.id} href={productsHref({ category: category.id })} className={`category-card ${category.tone}`}>
            <strong>{category.name}</strong><span>{category.description}</span>
          </Link>
        ))}
      </div>
    </section>
  );
}
