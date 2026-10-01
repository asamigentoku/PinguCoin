// 通信が一時的に失敗したときの、リトライ(指数バックオフ + ジッター)。
//
// 「失敗したら何でも再試行する」のは危険なので、次の方針で使う。
//   - 再試行してよいのは、一時的な失敗だけ。
//       ・つながらない(ネットワークエラー、サーバーの再起動中)
//       ・502 / 503(ゲートウェイやサーバーが、一時的に応答できない)
//     400 番台(入力の間違い、未ログイン、在庫不足など)は、何度やっても同じなので、すぐに諦める。
//     タイムアウト(504 を含む)も再試行しない(相手が遅いときに、さらに負荷をかけて悪化させないため)。
//   - 再試行してよいのは、同じ操作を2回やっても結果が変わらない(冪等な)ものだけ。
//       ・読み取り(GraphQL の query、GET)
//       ・冪等性キー(Idempotency-Key)が付いている注文
//       ・同じ名前への Blob の PUT(上書きなので、結果は同じ)
//     商品の作成・更新・削除のような書き込みは、再試行しない(1回目が実は成功していたら、二重に作られる)。
//   - 回数と待ち時間に上限がある。待ち時間は、少しずつ伸ばして、ランダムなばらつき(ジッター)を付ける
//     (多数のクライアントが、同時に再試行して、また同時に失敗するのを防ぐ)。
//
// このファイルは、ほかのファイルに依存しない(単体でテストできるように)。

/** HTTP のレスポンスが、失敗(2xx 以外)だったときのエラー。 */
export class HttpStatusError extends Error {
  status: number;

  constructor(status: number, message?: string) {
    super(message ?? `HTTP ${status}`);
    this.name = "HttpStatusError";
    this.status = status;
  }
}

/** 通信が、相手に届く前に失敗したときのエラー(接続できない、切れた)。 */
export class NetworkError extends Error {
  constructor(message = "ネットワークに接続できませんでした。") {
    super(message);
    this.name = "NetworkError";
  }
}

/** 一時的な失敗として、再試行してよい HTTP ステータス。 */
export const RETRYABLE_STATUS = [502, 503] as const;

/**
 * 再試行してよいエラーか。
 *  - NetworkError、および fetch の接続失敗(TypeError)
 *  - RETRYABLE_STATUS の HttpStatusError
 * タイムアウト(AbortError / TimeoutError / 504)や、400 番台は、再試行しない。
 */
export function isTransientError(error: unknown): boolean {
  if (error instanceof HttpStatusError) return (RETRYABLE_STATUS as readonly number[]).includes(error.status);
  if (error instanceof NetworkError) return true;
  // fetch は、接続できなかったとき、TypeError("fetch failed" など)を投げる。
  return error instanceof TypeError;
}

/**
 * Azure Blob への PUT(同じ名前への上書き。何回やっても結果は同じ)で、再試行してよいエラーか。
 * 接続失敗と、Azure が一時的に処理できないときの 500(内部エラー)/ 502 / 503(ServerBusy)。
 * 403(署名付き URL の期限切れ・不正)や 409 などは、何度やっても同じなので、再試行しない。
 */
export function isTransientStorageError(error: unknown): boolean {
  if (error instanceof HttpStatusError) return [500, 502, 503].includes(error.status);
  return error instanceof NetworkError || error instanceof TypeError;
}

export type RetryOptions = {
  /** 最初の1回も含めた、試行の最大回数。既定は 3。 */
  attempts?: number;
  /** 1回目の失敗のあとに待つ時間(ミリ秒。ジッターをかける前)。既定は 200。 */
  baseMs?: number;
  /** 待ち時間の上限(ミリ秒。ジッターをかける前)。既定は 2000。 */
  maxMs?: number;
  /** 再試行してよいエラーか。既定は isTransientError。 */
  shouldRetry?: (error: unknown) => boolean;
  /** 再試行する前に呼ばれる(attempt は、いま失敗した試行の番号。1 から)。 */
  onRetry?: (attempt: number, error: unknown, waitMs: number) => void;
  /** 待つ関数(テストで差し替える)。 */
  sleep?: (ms: number) => Promise<void>;
  /** 0 以上 1 未満の乱数(テストで差し替える)。 */
  random?: () => number;
  /** 中断のためのシグナル。中断されたら、待たずに、最後のエラーで終わる。 */
  signal?: AbortSignal;
};

/** attempt 回目(1 から)の失敗のあとの待ち時間(ジッターをかける前)。baseMs * 2^(attempt-1) で、maxMs を超えない。 */
export function backoffMs(attempt: number, baseMs = 200, maxMs = 2000): number {
  const exponent = Math.max(0, attempt - 1);
  return Math.min(maxMs, baseMs * 2 ** exponent);
}

const defaultSleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * fn を実行し、一時的な失敗(shouldRetry)のときだけ、上限の範囲で再試行する。
 * 待ち時間は、0〜backoffMs の間のランダムな値(フルジッター)。最後のエラーは、そのまま投げる。
 */
export async function withRetry<T>(fn: (attempt: number) => Promise<T>, options: RetryOptions = {}): Promise<T> {
  const attempts = Math.max(1, options.attempts ?? 3);
  const shouldRetry = options.shouldRetry ?? isTransientError;
  const sleep = options.sleep ?? defaultSleep;
  const random = options.random ?? Math.random;

  for (let attempt = 1; ; attempt++) {
    try {
      return await fn(attempt);
    } catch (error) {
      if (attempt >= attempts || !shouldRetry(error) || options.signal?.aborted) throw error;
      const waitMs = Math.floor(random() * backoffMs(attempt, options.baseMs, options.maxMs));
      options.onRetry?.(attempt, error, waitMs);
      await sleep(waitMs);
    }
  }
}
