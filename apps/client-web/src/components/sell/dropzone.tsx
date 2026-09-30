"use client";

import { useRef, useState } from "react";

// ファイルをドラッグ&ドロップ、またはクリックして選ぶ枠。
export function Dropzone({ title, hint, accept, multiple = false, disabled = false, onFiles }: {
  title: string;
  hint: string;
  accept?: string;
  multiple?: boolean;
  disabled?: boolean;
  onFiles: (files: File[]) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);

  function receive(list: FileList | null) {
    const files = Array.from(list ?? []);
    if (files.length) onFiles(multiple ? files : files.slice(0, 1));
    if (input.current) input.current.value = "";
  }

  return (
    <label
      className={`dropzone${dragging ? " is-dragging" : ""}${disabled ? " is-disabled" : ""}`}
      onDragOver={(e) => { e.preventDefault(); if (!disabled) setDragging(true); }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => { e.preventDefault(); setDragging(false); if (!disabled) receive(e.dataTransfer.files); }}
    >
      <svg width="30" height="30" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path d="M12 16V5m0 0-4 4m4-4 4 4M5 15v3a1.5 1.5 0 0 0 1.5 1.5h11A1.5 1.5 0 0 0 19 18v-3" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      <strong>{title}</strong>
      <span>{hint}</span>
      <input ref={input} type="file" accept={accept} multiple={multiple} disabled={disabled} onChange={(e) => receive(e.target.files)} />
    </label>
  );
}
