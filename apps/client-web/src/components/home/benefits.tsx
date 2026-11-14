import { useTranslations } from "next-intl";

export function Benefits() {
  const t = useTranslations("Home");
  const benefits = [1, 2, 3].map((number) => [t(`benefit${number}Title`), t(`benefit${number}Body`)]);
  return <section className="home-section shell" aria-label="Benefits"><div className="benefits">{benefits.map(([title, body]) => <div key={title}><strong>{title}</strong><p>{body}</p></div>)}</div></section>;
}
