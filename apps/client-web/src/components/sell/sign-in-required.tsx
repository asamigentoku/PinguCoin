import { EmptyState } from "@/components/common/empty-state";
import { PageHeader } from "@/components/common/page-header";

export function SignInRequired({ title }: { title: string }) {
  return (
    <>
      <PageHeader title={title} />
      <section className="shell sell-section">
        <EmptyState title="ログインが必要です" body="出品や出品の管理は、ログインしてから行えます。" href="/sign-in" action="ログインする" />
      </section>
    </>
  );
}
