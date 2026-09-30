import { connection } from "next/server";
import { UsersTable } from "@/components/users-table";
import { getUsers } from "@/lib/api";

export const metadata = { title: "ユーザー管理 | PinguCoin Admin" };

export default async function UsersPage() {
  await connection();
  let users: Awaited<ReturnType<typeof getUsers>> = [];
  let error: string | null = null;
  try {
    users = await getUsers();
  } catch {
    error = "pingu-apiに接続できませんでした。起動状態を確認してください。";
  }

  return (
    <main className="admin-main">
      <div className="admin-page-head">
        <div>
          <h1>ユーザー管理</h1>
          <p>PinguCoinに登録されているユーザーの一覧です。</p>
        </div>
      </div>
      {error ? (
        <div className="admin-empty">{error}</div>
      ) : users.length === 0 ? (
        <div className="admin-empty">登録されているユーザーがいません。</div>
      ) : (
        <UsersTable users={users} />
      )}
    </main>
  );
}
