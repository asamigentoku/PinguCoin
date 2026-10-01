// Package migrate は、起動時のマイグレーション(テーブルの作成・更新)を、複数の Pod が同時に実行しても衝突しないようにする。
//
// Kubernetes のデプロイ(ローリングアップデート)の間は、古い Pod と新しい Pod が同時に動く。
// どちらも起動時にマイグレーションを実行するので、そのままだと、同じテーブルを同時に作ろうとして失敗しうる。
// そこで、PostgreSQL のアドバイザリロックを取って、1つずつ実行する(先に取った Pod が終わるまで、ほかの Pod は待つ)。
package migrate

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// lockKey は、マイグレーション用のアドバイザリロックの番号(アプリ内で任意の固定値)。
// ロックは DB(データベース)ごとに独立しているので、3つのサービスが同じ値を使っても、互いに待たされない。
const lockKey int64 = 7262025001

// WithLock は、アドバイザリロックを取った状態で fn を実行する。
// ロックは接続(セッション)に紐づくので、ロックの取得・fn・解放を、同じ1つの接続の上で行う。
func WithLock(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	return db.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		if err := conn.Exec("SELECT pg_advisory_lock(?)", lockKey).Error; err != nil {
			return fmt.Errorf("acquire migration lock: %w", err)
		}
		// 接続が切れればロックは自動で解放されるので、解放の失敗は無視してよい。
		defer conn.Exec("SELECT pg_advisory_unlock(?)", lockKey)
		return fn(conn)
	})
}
