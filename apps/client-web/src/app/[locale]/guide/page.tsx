import { PageHeader } from "@/components/common/page-header";
import { Link } from "@/i18n/navigation";
import { getTranslations } from "next-intl/server";

export default async function GuidePage() {
  const t = await getTranslations("Guide");
  const steps = [1, 2, 3].map((number) => [t(`step${number}Title`), t(`step${number}Body`)]);
  return <><PageHeader title={t("title")} lead={t("lead")} /><section className="shell prose-section"><ol className="step-list">{steps.map(([title, body]) => <li key={title}><h2>{title}</h2><p>{body}</p></li>)}</ol><p className="prose-cta">{t("help")} <Link className="text-link" href="/contact">{t("contact")}</Link></p></section></>;
}
