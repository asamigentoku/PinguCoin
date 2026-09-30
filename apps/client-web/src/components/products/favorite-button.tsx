"use client";

import { useState } from "react";
import { HeartIcon } from "../ui/icons";

export function FavoriteButton({ name }: { name: string }) {
  const [active, setActive] = useState(false);
  return (
    <button className={`favorite-button ${active ? "is-active" : ""}`} onClick={() => setActive((v) => !v)} aria-pressed={active} aria-label={`${name}をお気に入りに追加`}>
      <HeartIcon />
    </button>
  );
}
