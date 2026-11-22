import { Link } from "@/i18n/navigation";

export function Brand() {
  return (
    <Link className="brand" href="/" aria-label="PinguCoin ホーム">
      <span className="brand-mark" aria-hidden="true">P</span>
      <span className="brand-name">PinguCoin</span>
    </Link>
  );
}
