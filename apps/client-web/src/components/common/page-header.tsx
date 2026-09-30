export function PageHeader({ title, lead, children }: { title: string; lead?: string; children?: React.ReactNode }) {
  return (
    <header className="page-header shell">
      <div><h1>{title}</h1>{lead && <p>{lead}</p>}</div>
      {children}
    </header>
  );
}
