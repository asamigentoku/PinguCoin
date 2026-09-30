"use client";

import { useState } from "react";
import { getDownloadUrl } from "@/app/purchases/actions";

export function DownloadButton({ assetId }: { assetId: number }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function download() {
    setBusy(true);
    setError("");
    const result = await getDownloadUrl(assetId);
    setBusy(false);
    if (!result.ok) return setError(result.error);
    window.location.href = result.url;
  }

  return (
    <>
      <button className="button button-coin download-button" onClick={download} disabled={busy}>{busy ? "準備中…" : "ダウンロード"}</button>
      {error && <small className="download-error" role="alert">{error}</small>}
    </>
  );
}
