import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { auth } from "@clerk/nextjs/server";
import { PageHeader } from "@/components/common/page-header";
import { ListingForm } from "@/components/sell/listing-form";
import { SignInRequired } from "@/components/sell/sign-in-required";
import { getMyProduct, getProductFiles } from "@/lib/seller";

export const metadata: Metadata = { title: "出品を編集する" };

export default async function EditListingPage({ params }: PageProps<"/sell/manage/[id]">) {
  const { userId } = await auth();
  if (!userId) return <SignInRequired title="出品を編集する" />;
  const id = Number((await params).id);
  if (!Number.isInteger(id) || id < 1) notFound();
  const product = await getMyProduct(id);
  if (!product) notFound();
  const files = await getProductFiles(id).catch(() => []);
  return (
    <>
      <PageHeader title="出品を編集する" lead={product.name} />
      <section className="shell sell-section">
        <ListingForm product={product} files={files} />
      </section>
    </>
  );
}
