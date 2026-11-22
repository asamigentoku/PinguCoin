import { NextRequest, NextResponse } from "next/server";

const MYMEMORY_URL = "https://api.mymemory.translated.net/get";
const MAX_TEXT_BYTES = 500;
const CACHE_TTL_MS = 24 * 60 * 60 * 1000;
const MAX_CACHE_ENTRIES = 1_000;
const supportedTargets = new Set(["en", "fr", "de", "zh-CN"]);

type CacheEntry = { translatedText: string; expiresAt: number };
type MyMemoryResponse = {
  responseData?: { translatedText?: string; match?: number };
  responseStatus?: number | string;
  responseDetails?: string;
};

const translationCache = new Map<string, CacheEntry>();

function cachedTranslation(key: string) {
  const entry = translationCache.get(key);
  if (!entry) return null;
  if (entry.expiresAt <= Date.now()) {
    translationCache.delete(key);
    return null;
  }
  return entry.translatedText;
}

function saveTranslation(key: string, translatedText: string) {
  if (translationCache.size >= MAX_CACHE_ENTRIES) {
    const oldestKey = translationCache.keys().next().value;
    if (oldestKey) translationCache.delete(oldestKey);
  }
  translationCache.set(key, { translatedText, expiresAt: Date.now() + CACHE_TTL_MS });
}

export async function GET(request: NextRequest) {
  const text = request.nextUrl.searchParams.get("text")?.trim() ?? "";
  const target = request.nextUrl.searchParams.get("target") ?? "";

  if (!text || !supportedTargets.has(target)) {
    return NextResponse.json({ message: "Invalid translation request" }, { status: 400 });
  }
  if (new TextEncoder().encode(text).byteLength > MAX_TEXT_BYTES) {
    return NextResponse.json({ message: "Text exceeds the 500-byte translation limit" }, { status: 413 });
  }

  const cacheKey = `ja|${target}|${text}`;
  const cached = cachedTranslation(cacheKey);
  if (cached) return NextResponse.json({ translatedText: cached, cached: true });

  const params = new URLSearchParams({ q: text, langpair: `ja|${target}`, mt: "1" });
  if (process.env.MYMEMORY_EMAIL) params.set("de", process.env.MYMEMORY_EMAIL);

  try {
    const response = await fetch(`${MYMEMORY_URL}?${params}`, {
      headers: { accept: "application/json" },
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
    });
    const payload = await response.json() as MyMemoryResponse;
    const translatedText = payload.responseData?.translatedText?.trim();
    if (!response.ok || Number(payload.responseStatus) !== 200 || !translatedText) {
      return NextResponse.json({ message: payload.responseDetails || "Translation failed" }, { status: 502 });
    }

    saveTranslation(cacheKey, translatedText);
    return NextResponse.json({ translatedText, cached: false }, {
      headers: { "Cache-Control": "private, max-age=86400" },
    });
  } catch {
    return NextResponse.json({ message: "Translation service is unavailable" }, { status: 502 });
  }
}
