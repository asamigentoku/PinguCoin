import { PageHeader } from "@/components/common/page-header";
import { ContactForm } from "@/components/forms/contact-form";
import { getTranslations } from "next-intl/server";

export default async function ContactPage() {
  const t = await getTranslations("Contact");
  return <><PageHeader title={t("title")} lead={t("lead")} /><section className="shell prose-section contact-section"><ContactForm /></section></>;
}
