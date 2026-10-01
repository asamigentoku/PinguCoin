// Package retry は、一時的な失敗を、指数バックオフ(待ち時間を少しずつ伸ばす)とジッター(待ち時間にばらつきを持たせる)で
// 再試行する。
//
// 「失敗したら何でも再試行する」のは危険なので、次の方針で使う。
//   - 再試行してよいのは、一時的な失敗(接続できない、相手が起動中、タイムアウトなど)だけ。
//     入力が間違っている、認証に失敗した、設定が壊れている、といった失敗は、何度やっても同じなので、
//     Permanent で包んで、すぐに諦める。
//   - 回数と待ち時間に上限を付ける(無限には再試行しない)。
//   - context が終わったら、すぐに止める。
//   - 再試行してよいのは、同じ操作を2回やっても結果が変わらない(冪等な)処理だけ。
//
// ジッターは、多数のクライアントが同時に失敗して、同時に再試行して、また同時に失敗する(再試行の集中)のを防ぐ。
package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// Policy は、再試行の回数と、待ち時間の決め方。
type Policy struct {
	// MaxAttempts は、最初の1回も含めた、試行の最大回数(1 なら、再試行しない)。
	MaxAttempts int
	// InitialBackoff は、1回目の失敗のあとに待つ時間(ジッターをかける前)。
	InitialBackoff time.Duration
	// MaxBackoff は、待ち時間の上限(ジッターをかける前)。
	MaxBackoff time.Duration
	// Multiplier は、失敗するたびに、待ち時間を何倍にするか(例: 2)。
	Multiplier float64
	// Jitter は、待ち時間にばらつきを持たせる関数。nil なら、0〜待ち時間の間の乱数(フルジッター)。
	Jitter func(backoff time.Duration) time.Duration
}

// Startup は、プロセスの起動時に、DB などへつなぐときの方針。合計で、最大 約15秒待つ。
var Startup = Policy{
	MaxAttempts:    6,
	InitialBackoff: 500 * time.Millisecond,
	MaxBackoff:     5 * time.Second,
	Multiplier:     2,
}

// Backoff は、attempt 回目(1 から数える)の失敗のあとに待つ時間(ジッターをかける前)を返す。
// InitialBackoff * Multiplier^(attempt-1) で、MaxBackoff を超えない。
func (p Policy) Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	multiplier := p.Multiplier
	if multiplier < 1 {
		multiplier = 1
	}
	backoff := float64(p.InitialBackoff)
	for i := 1; i < attempt; i++ {
		backoff *= multiplier
		if p.MaxBackoff > 0 && backoff >= float64(p.MaxBackoff) {
			backoff = float64(p.MaxBackoff)
			break
		}
	}
	if p.MaxBackoff > 0 && backoff > float64(p.MaxBackoff) {
		backoff = float64(p.MaxBackoff)
	}
	return time.Duration(backoff)
}

func (p Policy) wait(attempt int) time.Duration {
	backoff := p.Backoff(attempt)
	if backoff <= 0 {
		return 0
	}
	if p.Jitter != nil {
		return p.Jitter(backoff)
	}
	// フルジッター: 0〜backoff の間のランダムな時間。
	return time.Duration(rand.Int64N(int64(backoff) + 1))
}

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent は、「再試行しても無駄なエラー」であることを示す。これで包んだエラーが返ると、すぐに諦める。
// 返ってくるエラーは、包む前のものと errors.Is / errors.As で比べられる。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent は、err が Permanent で包まれているか。
func IsPermanent(err error) bool {
	var permanent *permanentError
	return errors.As(err, &permanent)
}

// Option は、Do の動きを調整する。
type Option func(*options)

type options struct {
	onRetry func(attempt int, err error, wait time.Duration)
	sleep   func(ctx context.Context, d time.Duration) error
}

// OnRetry は、再試行する前に呼ばれる(ログに出すためなど)。attempt は、いま失敗した試行の番号(1 から)。
func OnRetry(fn func(attempt int, err error, wait time.Duration)) Option {
	return func(o *options) { o.onRetry = fn }
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Do は、fn が成功するまで、方針(policy)の範囲で再試行する。
//   - fn が Permanent のエラーを返したら、すぐに諦めて、そのエラー(Permanent の包みは外す)を返す。
//   - MaxAttempts 回失敗したら、最後のエラーを返す。
//   - 待っている間に ctx が終わったら、最後のエラーと ctx のエラーを一緒に返す。
func Do(ctx context.Context, policy Policy, fn func(ctx context.Context) error, opts ...Option) error {
	settings := options{sleep: defaultSleep}
	for _, opt := range opts {
		opt(&settings)
	}
	attempts := policy.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return joinContextError(last, err)
		}
		last = fn(ctx)
		if last == nil {
			return nil
		}
		var permanent *permanentError
		if errors.As(last, &permanent) {
			return permanent.err
		}
		if attempt == attempts {
			break
		}

		wait := policy.wait(attempt)
		if settings.onRetry != nil {
			settings.onRetry(attempt, last, wait)
		}
		if err := settings.sleep(ctx, wait); err != nil {
			return joinContextError(last, err)
		}
	}
	return last
}

// DoValue は、Do の、値を返す版。
func DoValue[T any](ctx context.Context, policy Policy, fn func(ctx context.Context) (T, error), opts ...Option) (T, error) {
	var value T
	err := Do(ctx, policy, func(ctx context.Context) error {
		var err error
		value, err = fn(ctx)
		return err
	}, opts...)
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

func joinContextError(last, contextErr error) error {
	if last == nil {
		return contextErr
	}
	return fmt.Errorf("%w (retry stopped: %w)", last, contextErr)
}
