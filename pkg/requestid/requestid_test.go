package requestid

import (
	"context"
	"strings"
	"testing"
)

func TestNewIsUniqueAndValid(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := New()
		if len(id) != 32 || !Valid(id) {
			t.Fatalf("New() = %q is not a valid 32-character ID", id)
		}
		if seen[id] {
			t.Fatalf("New() returned %q twice", id)
		}
		seen[id] = true
	}
}

func TestValid(t *testing.T) {
	tests := map[string]bool{
		"abc123":                       true,
		"7f3c2b1a-9e8d-4c6b-a5f4-1234": true,
		"req_1.2-3":                    true,
		strings.Repeat("a", 128):       true,
		"":                             false,
		strings.Repeat("a", 129):       false,
		"has space":                    false,
		"line\nbreak":                  false,
		"tab\there":                    false,
		`quote"injection`:              false,
		"日本語":                          false,
		"semi;colon":                   false,
		"<script>":                     false,
		"id\r\nX-Injected: yes":        false,
	}
	for id, want := range tests {
		if got := Valid(id); got != want {
			t.Errorf("Valid(%q) = %v, want %v", id, got, want)
		}
	}
}

// 外から来た ID は、安全なものだけ使う。変なものは、使わずに、新しく作る(ログの偽装を防ぐ)。
func TestOrNew(t *testing.T) {
	if got := OrNew("from-client-123"); got != "from-client-123" {
		t.Errorf("a valid ID should be kept: %q", got)
	}
	for _, bad := range []string{"", "bad id", "line\nbreak", strings.Repeat("x", 500)} {
		got := OrNew(bad)
		if got == bad || !Valid(got) {
			t.Errorf("OrNew(%q) = %q; want a freshly generated valid ID", bad, got)
		}
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithContext(context.Background(), "abc")
	if got := FromContext(ctx); got != "abc" {
		t.Errorf("FromContext = %q", got)
	}
	if got := FromContext(context.Background()); got != "" {
		t.Errorf("an empty context should return an empty ID, got %q", got)
	}
}
