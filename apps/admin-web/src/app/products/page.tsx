import { connection } from "next/server";
import { ProductsTable } from "@/components/products-table";
import { getProducts } from "@/lib/api";

export const metadata = { title: "商品管理 | PinguCoin Admin" };

export default async function ProductsPage() {
  await connection();
  let products: Awaited<ReturnType<typeof getProducts>> = [];
  let error: string | null = null;
  try {
    products = await getProducts();
  } catch {
    error = "pingu-apiに接続できませんでした。起動状態を確認してください。";
  }

  return (
    <main className="admin-main">
      <div className="admin-page-head">
        <div>
          <h1>商品管理</h1>
          <p>出品されているすべての商品を確認できます。</p>
        </div>
      </div>
      {error ? (
        <div className="admin-empty">{error}</div>
      ) : products.length === 0 ? (
        <div className="admin-empty">登録されている商品がありません。</div>
      ) : (
        <ProductsTable products={products} />
      )}
    </main>
  );
}
