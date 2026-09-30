import type { Metadata } from "next";
import Link from "next/link";
import { auth } from "@clerk/nextjs/server";
import { EmptyState } from "@/components/common/empty-state";
import { PageHeader } from "@/components/common/page-header";
import { ManageList } from "@/components/sell/manage-list";
import { SignInRequired } from "@/components/sell/sign-in-required";
import { getMyProducts, getProductFiles } from "@/lib/seller";
import type { Product } from "@/lib/types";

export const metadata: Metadata = { title: "出品管理" };

export default async function ManagePage() {
  const { userId } = await auth();
  if (!userId) return <SignInRequired title="出品管理" />;

  let products: Product[] = [];
  const fileCounts: Record<number, number> = {};
  let apiAvailable = true;
  try {
    products = await getMyProducts();
    await Promise.all(products.map(async (product) => { fileCounts[product.id] = (await getProductFiles(product.id).catch(() => [])).length; }));
  } catch {
    apiAvailable = false;
  }

  return (
    <>
      <PageHeader title="出品管理" lead="出品した商品の編集、公開・非公開の切り替え、削除ができます。">
        <Link className="button button-coin" href="/sell">商品を出品する</Link>
      </PageHeader>
      <section className="shell sell-section">
        {!apiAvailable ? (
          <EmptyState title="出品を読み込めませんでした" body="pingu-apiが起動しているか確認して、ページを再読み込みしてください。" href="/sell/manage" action="再読み込みする" />
        ) : products.length === 0 ? (
          <EmptyState title="出品はまだありません" body="最初の作品を出品してみましょう。" href="/sell" action="商品を出品する" />
        ) : (
          <ManageList products={products} fileCounts={fileCounts} />
        )}
      </section>
    </>
  );
}
