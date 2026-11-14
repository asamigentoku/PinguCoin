import type { Metadata } from "next";
import { auth } from "@clerk/nextjs/server";
import { PageHeader } from "@/components/common/page-header";
import { ListingForm } from "@/components/sell/listing-form";
import { SignInRequired } from "@/components/sell/sign-in-required";

export const metadata: Metadata = { title: "商品を出品する" };

export default async function SellPage() {
  const { userId } = await auth();
  if (!userId) return <SignInRequired title="商品を出品する" />;
  return (
    <>
      <PageHeader title="商品を出品する" lead="作品の情報と、商品画像・販売するファイルを登録します。" />
      <section className="shell sell-section">
        <ListingForm />
      </section>
    </>
  );
}
