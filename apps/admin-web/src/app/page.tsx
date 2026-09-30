import Link from "next/link";

export default function Home() {
  return (
    <main className="admin-main">
      <div className="admin-page-head">
        <div>
          <h1>ダッシュボード</h1>
          <p>管理したい項目を上部メニューから選んでください。</p>
        </div>
      </div>
      <div className="admin-dashboard-grid">
        <Link className="admin-dashboard-card" href="/products">
          <span>商品管理</span>
          <p>出品されている商品の一覧を確認します。</p>
        </Link>
        <Link className="admin-dashboard-card" href="/users">
          <span>ユーザー管理</span>
          <p>登録されているユーザーの一覧を確認します。</p>
        </Link>
      </div>
    </main>
  );
}
