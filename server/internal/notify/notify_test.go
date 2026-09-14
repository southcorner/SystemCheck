package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestWebhookSend(t *testing.T) {
	var mu sync.Mutex
	var got AlertPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &got)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhook(srv.URL)
	if wh == nil {
		t.Fatal("expected non-nil webhook")
	}
	err := wh.Send(context.Background(), AlertPayload{MachineID: "m1", Severity: "warning", Message: "test"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got.Source != "systemcheck" || got.MachineID != "m1" || got.Message != "test" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestNilWebhookNoop(t *testing.T) {
	var wh *Webhook // disabled
	if err := wh.Send(context.Background(), AlertPayload{}); err != nil {
		t.Fatalf("nil webhook should be a no-op, got %v", err)
	}
}
