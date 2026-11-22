import { connection } from "next/server";
import { Benefits } from "@/components/home/benefits";
import { CategoryShowcase } from "@/components/home/category-showcase";
import { Hero } from "@/components/home/hero";
import { LatestProducts } from "@/components/home/latest-products";
import { Newsletter } from "@/components/home/newsletter";
import { getProducts } from "@/lib/api";
import type { Product } from "@/lib/types";

export default async function HomePage() {
  await connection();
  let products: Product[] = [];
  let apiAvailable = true;
  try {
    products = await getProducts();
  } catch {
    apiAvailable = false;
  }
  const latest = [...products].sort((a, b) => b.id - a.id).slice(0, 4);

  return (
    <>
      <Hero />
      <CategoryShowcase />
      <LatestProducts products={latest} apiAvailable={apiAvailable} />
      <Benefits />
      <Newsletter />
    </>
  );
}
