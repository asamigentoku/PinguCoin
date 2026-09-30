"use client";

import { useState } from "react";
import { formatDate } from "@/lib/format";
import type { User } from "@/lib/types";

const pageSize = 10;

export function UsersTable({ users }: { users: User[] }) {
  const [page, setPage] = useState(1);
  const pageCount = Math.max(1, Math.ceil(users.length / pageSize));
  const activePage = Math.min(page, pageCount);
  const pageUsers = users.slice((activePage - 1) * pageSize, activePage * pageSize);
  const goToPage = (next: number) => {
    setPage(Math.max(1, Math.min(next, pageCount)));
    document.querySelector("#admin-table-top")?.scrollIntoView({ behavior: "smooth" });
  };

  return (
    <div id="admin-table-top">
      <div className="result-summary">
        <span>{users.length}件中 {(activePage - 1) * pageSize + 1}–{Math.min(activePage * pageSize, users.length)}件を表示</span>
        <span>{activePage} / {pageCount} ページ</span>
      </div>
      <div className="admin-table-scroll">
        <table className="admin-table">
          <thead>
            <tr><th>ID</th><th>名前</th><th>メールアドレス</th><th>登録日</th></tr>
          </thead>
          <tbody>
            {pageUsers.map((user) => (
              <tr key={user.id}>
                <td>#{String(user.id).padStart(3, "0")}</td>
                <td>{user.name || "未設定"}</td>
                <td>{user.email}</td>
                <td>{formatDate(user.createdAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pageCount > 1 && (
        <nav className="pagination" aria-label="ユーザー一覧のページ">
          <button onClick={() => goToPage(activePage - 1)} disabled={activePage === 1}>← 前へ</button>
          <div>
            {Array.from({ length: pageCount }, (_, index) => index + 1).map((p) => (
              <button key={p} className={p === activePage ? "active" : ""} onClick={() => goToPage(p)} aria-current={p === activePage ? "page" : undefined}>{p}</button>
            ))}
          </div>
          <button onClick={() => goToPage(activePage + 1)} disabled={activePage === pageCount}>次へ →</button>
        </nav>
      )}
    </div>
  );
}
