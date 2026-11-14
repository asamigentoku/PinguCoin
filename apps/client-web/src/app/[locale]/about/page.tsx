import { PageHeader } from "@/components/common/page-header";
import { Newsletter } from "@/components/home/newsletter";
import { getTranslations } from "next-intl/server";

export default async function AboutPage() {
  const t = await getTranslations("About");
  const points = [1, 2, 3].map((number) => [t(`point${number}Title`), t(`point${number}Body`)]);
  return <><PageHeader title={t("title")} lead={t("lead")} /><section className="shell prose-section"><p className="prose-lead">{t("intro")}</p><div className="point-list">{points.map(([title, body]) => <div key={title}><h2>{title}</h2><p>{body}</p></div>)}</div></section><Newsletter /></>;
}
