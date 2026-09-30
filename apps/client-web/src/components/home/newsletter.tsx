import { NewsletterForm } from "../forms/newsletter-form";

export function Newsletter() {
  return (
    <section className="home-section shell">
      <div className="newsletter">
        <div><h2>新しい作品を、週末の受信箱へ。</h2><p>毎週金曜に、新着とおすすめのクリエイターをお届けします。</p></div>
        <NewsletterForm />
      </div>
    </section>
  );
}
