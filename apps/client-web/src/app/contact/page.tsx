import type { Metadata } from "next";
import { PageHeader } from "@/components/common/page-header";
import { ContactForm } from "@/components/forms/contact-form";

export const metadata: Metadata = { title: "お問い合わせ" };

export default function ContactPage() {
  return (
    <>
      <PageHeader title="お問い合わせ" lead="ご質問やご要望をお送りください。2営業日以内にご返信します。" />
      <section className="shell prose-section contact-section"><ContactForm /></section>
    </>
  );
}
