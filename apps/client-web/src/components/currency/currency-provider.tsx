"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";

import type { ExchangeRatePayload as Payload } from "@/lib/api";
const publicBffBase = (process.env.NEXT_PUBLIC_BFF_API_URL ?? "/api/v1").replace(/\/$/, "");
type CurrencyState = { rates: Record<string, number>; updatedAt: string | null; failed: boolean };
const CurrencyContext = createContext<CurrencyState>({ rates: {}, updatedAt: null, failed: false });

export function CurrencyProvider({ children, initialData }: { children: React.ReactNode; initialData: Payload | null }) {
  const [data, setData] = useState<Payload | null>(initialData);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    const refresh = () => fetch(`${publicBffBase}/exchange-rates`)
      .then((response) => response.ok ? response.json() as Promise<Payload> : Promise.reject())
      .then((payload) => { if (active) { setData(payload); setFailed(false); } })
      .catch(() => { if (active) setFailed(true); });
    refresh();
    const timer = window.setInterval(refresh, 180_000);
    return () => { active = false; window.clearInterval(timer); };
  }, []);

  const value = useMemo<CurrencyState>(() => ({
    rates: data?.rates ?? {},
    updatedAt: data?.updatedAt ?? null,
    failed,
  }), [data, failed]);
  return <CurrencyContext value={value}>{children}</CurrencyContext>;
}

export const useCurrency = () => useContext(CurrencyContext);
