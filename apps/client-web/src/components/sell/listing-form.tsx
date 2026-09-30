"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { confirmMainImage, confirmProductFile, createListing, prepareUploads, removeProductFile, updateListing } from "@/app/sell/actions";
import { categories } from "@/lib/categories";
import { formatBytes, listingStatuses, MAX_FILE_BYTES, MAX_IMAGE_BYTES, safeBlobName, validateListing, type ListingInput, type ListingStatus } from "@/lib/listing";
import type { ProductFile } from "@/lib/seller";
import type { Product } from "@/lib/types";
import { Dropzone } from "./dropzone";

type Target = { blobEndpoint: string; container: string; sasToken: string };
type Queued = { key: string; file: File; progress: number; state: "queued" | "uploading" | "done" | "error" };

// 署名付きURL(SAS)を使い、ブラウザからAzure Blobへ直接アップロードする(進捗を通知する)。
// 戻り値はSASを含まない素のBlob URL(pingu-apiのconfirmに渡す値)。
function putBlob(target: Target, prefix: string, file: File, onProgress: (percent: number) => void) {
  const url = `${target.blobEndpoint.replace(/\/$/, "")}/${target.container}/${prefix}${safeBlobName(file.name)}`;
  return new Promise<string>((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("PUT", `${url}?${target.sasToken.replace(/^\?/, "")}`);
    request.setRequestHeader("x-ms-blob-type", "BlockBlob");
    request.setRequestHeader("content-type", file.type || "application/octet-stream");
    request.upload.onprogress = (event) => event.lengthComputable && onProgress(Math.round((event.loaded / event.total) * 100));
    request.onload = () => (request.status >= 200 && request.status < 300 ? resolve(url) : reject(new Error(`アップロードに失敗しました(${request.status})。`)));
    request.onerror = () => reject(new Error("Azure Blobへ接続できませんでした。ストレージのCORS設定(許可するオリジン)を確認してください。"));
    request.send(file);
  });
}

export function ListingForm({ product, files = [] }: { product?: Product; files?: ProductFile[] }) {
  const router = useRouter();
  const editing = Boolean(product);
  const [savedId, setSavedId] = useState(product?.id);
  const [name, setName] = useState(product?.name ?? "");
  const [description, setDescription] = useState(product?.description ?? "");
  const [categoryId, setCategoryId] = useState(product?.categoryId ?? categories[0].id);
  const [price, setPrice] = useState(product ? String(product.price) : "");
  const [status, setStatus] = useState<ListingStatus>(product?.status === "draft" ? "draft" : "available");
  const [image, setImage] = useState<File | null>(null);
  const [existing, setExisting] = useState<ProductFile[]>(files);
  const [queue, setQueue] = useState<Queued[]>([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const previewUrl = useMemo(() => (image ? URL.createObjectURL(image) : product?.imageUrl), [image, product?.imageUrl]);
  useEffect(() => () => { if (image && previewUrl) URL.revokeObjectURL(previewUrl); }, [image, previewUrl]);

  const patch = (key: string, change: Partial<Queued>) => setQueue((current) => current.map((item) => (item.key === key ? { ...item, ...change } : item)));

  function addFiles(list: File[]) {
    setError("");
    const tooBig = list.filter((file) => file.size > MAX_FILE_BYTES);
    if (tooBig.length) setError(`1ファイルは${formatBytes(MAX_FILE_BYTES)}までです: ${tooBig.map((file) => file.name).join("、")}`);
    setQueue((current) => [...current, ...list.filter((file) => file.size <= MAX_FILE_BYTES).map((file) => ({ key: crypto.randomUUID(), file, progress: 0, state: "queued" as const }))]);
  }

  function pickImage(list: File[]) {
    const file = list[0];
    if (!file.type.startsWith("image/")) return setError("画像ファイル(JPG・PNGなど)を選んでください。");
    if (file.size > MAX_IMAGE_BYTES) return setError(`画像は${formatBytes(MAX_IMAGE_BYTES)}以下にしてください。`);
    setError("");
    setImage(file);
  }

  async function removeExisting(file: ProductFile) {
    if (!confirm(`「${file.originalFilename}」を削除しますか？ 元に戻せません。`)) return;
    setError("");
    const result = await removeProductFile(file.id);
    if (!result.ok) return setError(result.error);
    setExisting((current) => current.filter((item) => item.id !== file.id));
  }

  function checkBeforeSave() {
    if (status === "available" && !(image || product?.imageUrl)) return "公開するには商品画像が必要です。下書きなら画像なしで保存できます。";
    if (status === "available" && existing.length + queue.length === 0) return "公開するには商品ファイルが1つ以上必要です。下書きならファイルなしで保存できます。";
    return null;
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    const input: ListingInput = { name, description, categoryId, price: Number(price), status };
    const invalid = validateListing(input) ?? checkBeforeSave();
    if (invalid) return setError(invalid);

    try {
      setBusy("商品を保存しています…");
      // 新規のときは、アップロードが全部終わるまで下書きにしておく(途中で失敗しても公開されない)。
      const draftFirst = !editing;
      let id = savedId;
      if (id) {
        const updated = await updateListing(id, draftFirst ? { ...input, status: "draft" } : input);
        if (!updated.ok) throw new Error(updated.error);
      } else {
        const created = await createListing({ ...input, status: "draft" });
        if (!created.ok) throw new Error(created.error);
        id = created.data.productId;
        setSavedId(id);
      }

      const pending = queue.filter((item) => item.state !== "done");
      if (image || pending.length) {
        setBusy("アップロードの準備をしています…");
        const targets = await prepareUploads(id);
        if (!targets.ok) throw new Error(targets.error);

        if (image) {
          setBusy("商品画像をアップロードしています…");
          const url = await putBlob(targets.data.image, targets.data.image.mainImagePrefix, image, () => undefined);
          const confirmed = await confirmMainImage(id, url, product?.imageUrl);
          if (!confirmed.ok) throw new Error(confirmed.error);
          setImage(null);
        }

        let order = existing.reduce((max, file) => Math.max(max, file.sortOrder), 0);
        for (const [index, item] of pending.entries()) {
          setBusy(`商品ファイルをアップロードしています…(${index + 1}/${pending.length})`);
          patch(item.key, { state: "uploading", progress: 0 });
          try {
            const url = await putBlob(targets.data.files, targets.data.files.pathPrefix, item.file, (progress) => patch(item.key, { progress }));
            const confirmed = await confirmProductFile(id, url, { originalFilename: item.file.name, contentType: item.file.type || "application/octet-stream", fileSize: item.file.size, sortOrder: ++order });
            if (!confirmed.ok) throw new Error(confirmed.error);
            setExisting((current) => [...current, confirmed.data]);
            setQueue((current) => current.filter((queued) => queued.key !== item.key));
          } catch (cause) {
            patch(item.key, { state: "error" });
            throw cause;
          }
        }
      }

      if (draftFirst && status !== "draft") {
        const published = await updateListing(id, input);
        if (!published.ok) throw new Error(published.error);
      }
      router.push("/sell/manage");
      router.refresh();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "保存に失敗しました。";
      setError(`${message} ${editing ? "" : "商品は下書きとして保存されています。もう一度「出品する」を押すと、続きからやり直せます。"}`.trim());
      setBusy("");
    }
  }

  const uploading = Boolean(busy);
  return (
    <form className="listing-form" onSubmit={submit}>
      <section className="form-card">
        <h2><span>1</span>基本情報</h2>
        <label>商品名<input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} placeholder="例: ペンギンの壁紙セット" required /></label>
        <label>説明<textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={5} placeholder="作品の内容や使い方を書いてください。" /></label>
        <div className="listing-row">
          <label>カテゴリー
            <select value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))}>
              {categories.map((category) => <option key={category.id} value={category.id}>{category.name}</option>)}
            </select>
          </label>
          <label>価格(ポイント)<input type="number" inputMode="numeric" min={0} step={1} value={price} onChange={(e) => setPrice(e.target.value)} placeholder="500" required /></label>
        </div>
      </section>

      <section className="form-card">
        <h2><span>2</span>商品画像</h2>
        <p className="form-help">商品一覧やページの一番上に表示される画像です。JPG・PNGなど、{formatBytes(MAX_IMAGE_BYTES)}まで。</p>
        {previewUrl ? (
          <div className="image-picked">
            <div className="upload-preview" style={{ backgroundImage: `url(${previewUrl})` }} role="img" aria-label="商品画像のプレビュー" />
            <div>
              <strong>{image ? image.name : "登録済みの画像"}</strong>
              {image && <small>{formatBytes(image.size)} ・ 保存すると反映されます</small>}
              <div className="image-picked-actions">
                <label className="mini-button">別の画像にする<input type="file" accept="image/*" hidden disabled={uploading} onChange={(e) => e.target.files && e.target.files.length > 0 && pickImage(Array.from(e.target.files))} /></label>
                {image && <button type="button" className="mini-button" onClick={() => setImage(null)} disabled={uploading}>選択を取り消す</button>}
              </div>
            </div>
          </div>
        ) : (
          <Dropzone title="画像をここにドラッグ&ドロップ" hint="またはクリックしてファイルを選ぶ" accept="image/*" disabled={uploading} onFiles={pickImage} />
        )}
      </section>

      <section className="form-card">
        <h2><span>3</span>商品ファイル<em>購入した人が受け取るデータ</em></h2>
        <p className="form-help">何個でも追加できます(1ファイル{formatBytes(MAX_FILE_BYTES)}まで)。非公開で保存され、購入者だけがダウンロードできます。</p>
        <Dropzone title="ファイルをここにドラッグ&ドロップ" hint="またはクリックして選ぶ(複数選択できます)" multiple disabled={uploading} onFiles={addFiles} />

        {existing.length + queue.length > 0 && (
          <ul className="file-list" aria-label="商品ファイル">
            {existing.map((file) => (
              <li key={`saved-${file.id}`}>
                <span className="file-icon" aria-hidden="true">✓</span>
                <div><strong>{file.originalFilename}</strong><small>{formatBytes(file.fileSize)} ・ アップロード済み</small></div>
                <button type="button" className="file-remove" onClick={() => removeExisting(file)} disabled={uploading}>削除</button>
              </li>
            ))}
            {queue.map((item) => (
              <li key={item.key} className={`is-${item.state}`}>
                <span className="file-icon" aria-hidden="true">{item.state === "error" ? "!" : "↑"}</span>
                <div>
                  <strong>{item.file.name}</strong>
                  <small>{formatBytes(item.file.size)} ・ {item.state === "queued" ? "保存するとアップロードします" : item.state === "uploading" ? `アップロード中 ${item.progress}%` : "失敗しました(もう一度保存すると再試行します)"}</small>
                  {item.state === "uploading" && <progress max={100} value={item.progress} />}
                </div>
                <button type="button" className="file-remove" onClick={() => setQueue((current) => current.filter((queued) => queued.key !== item.key))} disabled={item.state === "uploading"}>取り消し</button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="form-card">
        <h2><span>4</span>公開設定</h2>
        <div className="status-cards">
          {listingStatuses.map((item) => (
            <label key={item.value} className={status === item.value ? "is-selected" : undefined}>
              <input type="radio" name="status" value={item.value} checked={status === item.value} onChange={() => setStatus(item.value)} />
              <strong>{item.label}</strong>
              <small>{item.value === "available" ? "商品一覧に表示され、購入できます。" : "自分だけが見られます。準備ができたら公開できます。"}</small>
            </label>
          ))}
        </div>
      </section>

      {error && <p className="form-error" role="alert">{error}</p>}
      <div className="listing-actions">
        <button className="button button-coin" type="submit" disabled={uploading}>{busy || (editing ? "変更を保存する" : "出品する")}</button>
        <button className="button button-ghost" type="button" onClick={() => router.push("/sell/manage")} disabled={uploading}>キャンセル</button>
      </div>
    </form>
  );
}
