package reqcontext

import (
	"context"
	"testing"
)

func TestUserRoundTrip(t *testing.T) {
	claims := &Claims{UserID: 7, ClerkUserID: "user_abc", Email: "a@example.com", Name: "A"}

	got, ok := UserFromContext(WithUser(context.Background(), claims))

	if !ok || got != claims {
		t.Errorf("UserFromContext = %v, %v; want the same claims", got, ok)
	}
}

func TestNoUserMeansNotLoggedIn(t *testing.T) {
	if got, ok := UserFromContext(context.Background()); ok || got != nil {
		t.Errorf("an empty context returned %v, %v", got, ok)
	}
}

// nilのClaimsを積んでも、ログイン済みとは扱わない(nilを逆参照してパニックしない)。
func TestNilClaimsAreNotALogin(t *testing.T) {
	if got, ok := UserFromContext(WithUser(context.Background(), nil)); ok || got != nil {
		t.Errorf("nil claims were treated as a login: %v, %v", got, ok)
	}
}

func TestInnerUserOverridesTheOuterOne(t *testing.T) {
	outer := WithUser(context.Background(), &Claims{UserID: 1})
	inner := WithUser(outer, &Claims{UserID: 2})

	if got, _ := UserFromContext(inner); got.UserID != 2 {
		t.Errorf("UserID = %d, want 2", got.UserID)
	}
	if got, _ := UserFromContext(outer); got.UserID != 1 {
		t.Errorf("the outer context was modified: %d", got.UserID)
	}
}
