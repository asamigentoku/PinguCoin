// node の標準のテストランナーで動かす(npm test)。
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { HttpStatusError, NetworkError, RETRYABLE_STATUS, backoffMs, isTransientError, isTransientStorageError, withRetry } from "./retry.ts";

// 待たずに、待ち時間だけを記録する。乱数は、常に最大(ジッターで、待ち時間が最大になる)にして、計算を確かめる。
function recorder() {
  const waits: number[] = [];
  return {
    waits,
    options: {
      sleep: async (ms: number) => {
        waits.push(ms);
      },
      random: () => 0.999999,
    },
  };
}

describe("isTransientError", () => {
  it("retries connection failures", () => {
    assert.equal(isTransientError(new NetworkError()), true);
    assert.equal(isTransientError(new TypeError("fetch failed")), true);
  });

  it("retries only 502 / 503", () => {
    for (const status of RETRYABLE_STATUS) assert.equal(isTransientError(new HttpStatusError(status)), true, String(status));
    for (const status of [400, 401, 403, 404, 409, 422, 429, 500, 501, 504]) {
      assert.equal(isTransientError(new HttpStatusError(status)), false, `${status} must not be retried`);
    }
  });

  it("does not retry timeouts or unknown errors", () => {
    assert.equal(isTransientError(new DOMException("timed out", "TimeoutError")), false);
    assert.equal(isTransientError(new DOMException("aborted", "AbortError")), false);
    assert.equal(isTransientError(new Error("boom")), false);
    assert.equal(isTransientError("string"), false);
  });
});

describe("isTransientStorageError", () => {
  it("retries connection failures and Azure's temporary 5xx", () => {
    assert.equal(isTransientStorageError(new NetworkError()), true);
    for (const status of [500, 502, 503]) assert.equal(isTransientStorageError(new HttpStatusError(status)), true, String(status));
  });

  it("does not retry an expired or invalid signature, or a conflict", () => {
    for (const status of [400, 403, 404, 409, 412]) assert.equal(isTransientStorageError(new HttpStatusError(status)), false, String(status));
    assert.equal(isTransientStorageError(new Error("boom")), false);
  });
});

describe("backoffMs", () => {
  it("doubles each time and is capped", () => {
    assert.deepEqual([1, 2, 3, 4, 5, 6].map((attempt) => backoffMs(attempt, 200, 2000)), [200, 400, 800, 1600, 2000, 2000]);
  });
});

describe("withRetry", () => {
  it("returns the result without retrying when it succeeds", async () => {
    let calls = 0;
    const result = await withRetry(async () => {
      calls++;
      return "ok";
    });
    assert.equal(result, "ok");
    assert.equal(calls, 1);
  });

  it("retries a transient failure until it succeeds, waiting longer each time", async () => {
    const { waits, options } = recorder();
    let calls = 0;

    const result = await withRetry(async () => {
      calls++;
      if (calls < 3) throw new HttpStatusError(503);
      return "ok";
    }, options);

    assert.equal(result, "ok");
    assert.equal(calls, 3);
    assert.deepEqual(waits, [199, 399]); // 200ms と 400ms の、ジッターで最大のとき(0.999999 倍)
  });

  it("gives up after the maximum number of attempts and throws the last error", async () => {
    const { waits, options } = recorder();
    let calls = 0;

    await assert.rejects(
      withRetry(async () => {
        calls++;
        throw new HttpStatusError(502, `attempt ${calls}`);
      }, { ...options, attempts: 3 }),
      (error: unknown) => error instanceof HttpStatusError && error.message === "attempt 3",
    );

    assert.equal(calls, 3);
    assert.equal(waits.length, 2, "no wait after the last attempt");
  });

  it("does not retry errors that will not change on a retry", async () => {
    const { waits, options } = recorder();
    let calls = 0;

    await assert.rejects(
      withRetry(async () => {
        calls++;
        throw new HttpStatusError(409);
      }, options),
      HttpStatusError,
    );

    assert.equal(calls, 1);
    assert.deepEqual(waits, []);
  });

  it("keeps the wait within 0..backoff (jitter)", async () => {
    const waits: number[] = [];
    for (const random of [0, 0.25, 0.5, 0.999999]) {
      await assert.rejects(
        withRetry(async () => {
          throw new NetworkError();
        }, { attempts: 2, baseMs: 1000, maxMs: 1000, random: () => random, sleep: async (ms) => void waits.push(ms) }),
      );
    }
    assert.deepEqual(waits, [0, 250, 500, 999]);
  });

  it("takes a custom shouldRetry", async () => {
    const { options } = recorder();
    let calls = 0;
    await assert.rejects(
      withRetry(async () => {
        calls++;
        throw new HttpStatusError(500);
      }, { ...options, shouldRetry: (error) => error instanceof HttpStatusError && error.status === 500, attempts: 2 }),
    );
    assert.equal(calls, 2);
  });

  it("calls onRetry before each retry", async () => {
    const { options } = recorder();
    const seen: number[] = [];
    await assert.rejects(
      withRetry(async () => {
        throw new NetworkError();
      }, { ...options, attempts: 3, onRetry: (attempt) => seen.push(attempt) }),
    );
    assert.deepEqual(seen, [1, 2]);
  });

  it("stops retrying when aborted", async () => {
    const { options } = recorder();
    const controller = new AbortController();
    let calls = 0;
    await assert.rejects(
      withRetry(async () => {
        calls++;
        controller.abort();
        throw new NetworkError();
      }, { ...options, signal: controller.signal }),
    );
    assert.equal(calls, 1);
  });

  it("passes the attempt number to the function", async () => {
    const { options } = recorder();
    const attempts: number[] = [];
    await withRetry(async (attempt) => {
      attempts.push(attempt);
      if (attempt < 2) throw new NetworkError();
    }, options);
    assert.deepEqual(attempts, [1, 2]);
  });
});
