import Link from "next/link";

export function EmptyState({ title, body, href, action }: { title: string; body: string; href?: string; action?: string }) {
  return (
    <div className="empty-state">
      <span className="empty-coin" aria-hidden="true">P</span>
      <h2>{title}</h2>
      <p>{body}</p>
      {href && action && <Link className="button button-primary" href={href}>{action}</Link>}
    </div>
  );
}
