package forex

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunFetchesOnceAndPublishesLatestSnapshotRepeatedly(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		if got, want := r.URL.Query().Get("symbol"), "USD/JPY,EUR/JPY,GBP/JPY,CNY/JPY"; got != want {
			t.Errorf("symbol query = %q, want %q", got, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"USD/JPY": map[string]string{"price": "150.25"},
			"EUR/JPY": map[string]string{"price": "160.25"},
			"GBP/JPY": map[string]string{"price": "190.25"},
			"CNY/JPY": map[string]string{"price": "20.25"},
		})
	}))
	defer upstream.Close()

	service := New("test-key", upstream.URL, slog.New(slog.DiscardHandler))
	published := make(chan Snapshot, 4)
	service.SetPublisher(func(_ context.Context, snapshot Snapshot) error {
		published <- snapshot
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.run(ctx, time.Hour, 10*time.Millisecond)

	var firstUpdatedAt time.Time
	for i := 0; i < 3; i++ {
		select {
		case snapshot := <-published:
			if i == 0 {
				firstUpdatedAt = snapshot.UpdatedAt
			} else if !snapshot.UpdatedAt.Equal(firstUpdatedAt) {
				t.Fatalf("publish %d fetched rates again: updatedAt = %s, want %s", i, snapshot.UpdatedAt, firstUpdatedAt)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for snapshot publication")
		}
	}
	cancel()

	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}
