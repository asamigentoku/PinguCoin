package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errTemporary = errors.New("temporarily unavailable")

// ジッターなし・待たずに、待ち時間だけを記録するための設定。
func recordingOptions(waits *[]time.Duration) []Option {
	return []Option{func(o *options) {
		o.sleep = func(_ context.Context, d time.Duration) error {
			*waits = append(*waits, d)
			return nil
		}
	}}
}

func noJitter(backoff time.Duration) time.Duration { return backoff }

func TestSucceedsWithoutRetrying(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Policy{MaxAttempts: 3}, func(context.Context) error { calls++; return nil })
	if err != nil || calls != 1 {
		t.Errorf("err=%v calls=%d, want nil and 1", err, calls)
	}
}

func TestRetriesUntilItSucceeds(t *testing.T) {
	var waits []time.Duration
	calls := 0
	policy := Policy{MaxAttempts: 5, InitialBackoff: 100 * time.Millisecond, MaxBackoff: time.Second, Multiplier: 2, Jitter: noJitter}

	err := Do(context.Background(), policy, func(context.Context) error {
		calls++
		if calls < 3 {
			return errTemporary
		}
		return nil
	}, recordingOptions(&waits)...)

	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d, want nil and 3", err, calls)
	}
	// 失敗のたびに、待ち時間が伸びる(100ms → 200ms)。
	if len(waits) != 2 || waits[0] != 100*time.Millisecond || waits[1] != 200*time.Millisecond {
		t.Errorf("waits = %v, want [100ms 200ms]", waits)
	}
}

// 回数に上限がある。無限には再試行しない。最後のエラーを返す。
func TestGivesUpAfterMaxAttempts(t *testing.T) {
	var waits []time.Duration
	calls := 0
	policy := Policy{MaxAttempts: 4, InitialBackoff: time.Millisecond, Multiplier: 2, Jitter: noJitter}

	err := Do(context.Background(), policy, func(context.Context) error { calls++; return errTemporary }, recordingOptions(&waits)...)

	if !errors.Is(err, errTemporary) {
		t.Errorf("err = %v, want the last error", err)
	}
	if calls != 4 {
		t.Errorf("calls = %d, want 4", calls)
	}
	if len(waits) != 3 {
		t.Errorf("waited %d times, want 3 (no wait after the last attempt)", len(waits))
	}
}

// 再試行しても無駄なエラー(設定の間違い、認証の失敗など)は、すぐに諦める。
func TestPermanentErrorsAreNotRetried(t *testing.T) {
	var waits []time.Duration
	calls := 0
	bad := errors.New("invalid dsn")

	err := Do(context.Background(), Policy{MaxAttempts: 5, InitialBackoff: time.Millisecond}, func(context.Context) error {
		calls++
		return Permanent(bad)
	}, recordingOptions(&waits)...)

	if calls != 1 || len(waits) != 0 {
		t.Errorf("calls=%d waits=%v, want a single attempt", calls, waits)
	}
	if !errors.Is(err, bad) {
		t.Errorf("err = %v, want the original error", err)
	}
	if IsPermanent(err) {
		t.Error("the Permanent wrapper should be removed from the returned error")
	}
}

func TestPermanentWrapsAndUnwraps(t *testing.T) {
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) should be nil")
	}
	wrapped := Permanent(errTemporary)
	if !IsPermanent(wrapped) || !errors.Is(wrapped, errTemporary) || wrapped.Error() != errTemporary.Error() {
		t.Errorf("unexpected wrapper: %v", wrapped)
	}
	if IsPermanent(errTemporary) {
		t.Error("a plain error is not permanent")
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	policy := Policy{InitialBackoff: 100 * time.Millisecond, MaxBackoff: time.Second, Multiplier: 2}
	want := []time.Duration{100, 200, 400, 800, 1000, 1000, 1000}
	for i, ms := range want {
		if got := policy.Backoff(i + 1); got != ms*time.Millisecond {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, ms*time.Millisecond)
		}
	}
	// 倍率が1未満でも、待ち時間が縮まない。attempt が0以下でも、1回目として扱う。
	flat := Policy{InitialBackoff: 50 * time.Millisecond, Multiplier: 0.5}
	if flat.Backoff(5) != 50*time.Millisecond || flat.Backoff(0) != 50*time.Millisecond {
		t.Errorf("unexpected flat backoff: %v / %v", flat.Backoff(5), flat.Backoff(0))
	}
}

// 既定のジッターは、0〜待ち時間の間に収まる(同時に再試行が集中するのを防ぐ)。
func TestDefaultJitterStaysWithinTheBackoff(t *testing.T) {
	policy := Policy{InitialBackoff: 100 * time.Millisecond, MaxBackoff: time.Second, Multiplier: 2}
	varied := map[time.Duration]bool{}
	for i := 0; i < 200; i++ {
		wait := policy.wait(3) // 待ち時間(ジッターなし)は 400ms
		if wait < 0 || wait > 400*time.Millisecond {
			t.Fatalf("wait %v is outside 0..400ms", wait)
		}
		varied[wait] = true
	}
	if len(varied) < 20 {
		t.Errorf("only %d distinct waits in 200 tries; the jitter is not random", len(varied))
	}
}

func TestStopsWaitingWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	policy := Policy{MaxAttempts: 10, InitialBackoff: time.Hour, Jitter: noJitter}

	done := make(chan error, 1)
	go func() {
		done <- Do(ctx, policy, func(context.Context) error { calls++; return errTemporary })
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, errTemporary) {
			t.Errorf("err = %v, want both the last error and the cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Do kept waiting after the context was canceled")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoesNotStartWhenTheContextIsAlreadyDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0

	err := Do(ctx, Policy{MaxAttempts: 3}, func(context.Context) error { calls++; return nil })

	if calls != 0 || !errors.Is(err, context.Canceled) {
		t.Errorf("calls=%d err=%v", calls, err)
	}
}

func TestOnRetryIsCalledBeforeEachRetry(t *testing.T) {
	var waits []time.Duration
	var seen []int
	policy := Policy{MaxAttempts: 3, InitialBackoff: time.Millisecond, Multiplier: 2, Jitter: noJitter}

	_ = Do(context.Background(), policy, func(context.Context) error { return errTemporary },
		append(recordingOptions(&waits), OnRetry(func(attempt int, err error, wait time.Duration) {
			if !errors.Is(err, errTemporary) {
				t.Errorf("OnRetry got %v", err)
			}
			seen = append(seen, attempt)
		}))...)

	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Errorf("OnRetry attempts = %v, want [1 2]", seen)
	}
}

func TestMaxAttemptsBelowOneStillRunsOnce(t *testing.T) {
	calls := 0
	_ = Do(context.Background(), Policy{}, func(context.Context) error { calls++; return errTemporary })
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoValue(t *testing.T) {
	var waits []time.Duration
	calls := 0
	value, err := DoValue(context.Background(), Policy{MaxAttempts: 3, InitialBackoff: time.Millisecond, Jitter: noJitter},
		func(context.Context) (string, error) {
			calls++
			if calls < 2 {
				return "", errTemporary
			}
			return "connected", nil
		}, recordingOptions(&waits)...)
	if err != nil || value != "connected" || calls != 2 {
		t.Errorf("value=%q err=%v calls=%d", value, err, calls)
	}

	failed, err := DoValue(context.Background(), Policy{MaxAttempts: 2, Jitter: noJitter},
		func(context.Context) (int, error) { return 42, errTemporary }, recordingOptions(&waits)...)
	if !errors.Is(err, errTemporary) || failed != 0 {
		t.Errorf("a failed DoValue must return the zero value: %d, %v", failed, err)
	}
}
