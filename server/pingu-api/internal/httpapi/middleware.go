package httpapi

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/clerkauth"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/reqcontext"
)

// WithOptionalAuth はAuthorizationヘッダ(Bearer)にClerkのセッショントークンがあれば検証し、
// 成功したらorcan-apiのユーザープロフィールに解決してcontextに積む。
// トークンが無い/不正でもリクエスト自体は通す(GraphQLは公開クエリと認証必須のフィールド(me等)が
// 混在するため、必須化は各resolver/handler側で行う)。
func WithOptionalAuth(orcan *orcanclient.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			token := extractBearerToken(req)
			if token == "" {
				next.ServeHTTP(w, req)
				return
			}

			clerkUserID, err := clerkauth.VerifySessionToken(req.Context(), token)
			if err != nil {
				next.ServeHTTP(w, req)
				return
			}

			claims, err := resolveUser(req.Context(), orcan, clerkUserID)
			if err != nil {
				next.ServeHTTP(w, req)
				return
			}

			ctx := reqcontext.WithUser(req.Context(), claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// resolveUser はClerkのユーザーIDから、orcan-api側のアプリ内プロフィール(users.id等)を解決する。
// 既に存在すればそれをそのまま使い(Clerk APIへの追加呼び出しは発生しない)、
// 初回アクセスの場合のみClerkのBackend APIからemail/nameを取得してorcan-apiにプロフィールを作成する。
func resolveUser(ctx context.Context, orcan *orcanclient.Client, clerkUserID string) (*reqcontext.Claims, error) {
	resp, err := orcan.User.GetUserByClerkID(ctx, &orcanpb.GetUserByClerkIDRequest{ClerkUserId: clerkUserID})
	if err == nil {
		return claimsFromPB(resp.GetUser()), nil
	}
	if status.Code(err) != codes.NotFound {
		return nil, err
	}

	email, name, err := clerkauth.FetchProfile(ctx, clerkUserID)
	if err != nil {
		return nil, err
	}

	ensureResp, err := orcan.User.EnsureUser(ctx, &orcanpb.EnsureUserRequest{
		ClerkUserId: clerkUserID,
		Email:       email,
		Name:        name,
	})
	if err != nil {
		return nil, err
	}
	return claimsFromPB(ensureResp.GetUser()), nil
}

func claimsFromPB(u *orcanpb.User) *reqcontext.Claims {
	return &reqcontext.Claims{
		UserID:      uint(u.GetId()),
		ClerkUserID: u.GetClerkUserId(),
		Email:       u.GetEmail(),
		Name:        u.GetName(),
	}
}

func extractBearerToken(req *http.Request) string {
	h := req.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
