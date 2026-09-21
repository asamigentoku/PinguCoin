// Package clerkauth はClerk(フロントエンドで完結する認証基盤)のセッショントークン検証と、
// 初回アクセス時のプロフィール取得(email/name)を扱う。
// pingu-api/orcan-apiはパスワード等の認証情報を一切保持せず、ここでの検証結果(Clerkのユーザー ID)
// を元に orcan-api の users テーブル(アプリ内プロフィール)と突き合わせる。
package clerkauth

import (
	"context"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

// Init はClerkのSecret Keyを設定する。プロセス起動時に一度だけ呼ぶ。
func Init(secretKey string) {
	clerk.SetKey(secretKey)
}

// VerifySessionToken はClerkのセッショントークン(JWT)を検証し、ClerkのユーザーID(sub claim)を返す。
// 署名検証にはClerkのJWKS(公開鍵)を使う(Clerk側が管理する秘密鍵で署名されているため、
// pingu-api/orcan-apiは検証専用の秘密鍵を持つ必要がない)。
func VerifySessionToken(ctx context.Context, token string) (string, error) {
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{Token: token})
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// FetchProfile はClerkのBackend APIからユーザーのメールアドレス・氏名を取得する。
// orcan-apiにまだプロフィールが無い(初回アクセス)場合の作成時のみ呼ぶ
// (毎リクエストでClerk APIを叩かないように、既存ユーザーの解決はorcan-api側のDBで完結させる)。
func FetchProfile(ctx context.Context, clerkUserID string) (email, name string, err error) {
	u, err := user.Get(ctx, clerkUserID)
	if err != nil {
		return "", "", err
	}

	if u.PrimaryEmailAddressID != nil {
		for _, e := range u.EmailAddresses {
			if e.ID == *u.PrimaryEmailAddressID {
				email = e.EmailAddress
				break
			}
		}
	}

	var first, last string
	if u.FirstName != nil {
		first = *u.FirstName
	}
	if u.LastName != nil {
		last = *u.LastName
	}
	name = strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))

	return email, name, nil
}
