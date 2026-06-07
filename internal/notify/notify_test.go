package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// noGuard disables the SSRF check so tests can target httptest (127.0.0.1).
func noGuard(context.Context, string) error { return nil }

func TestSendGenericPayload(t *testing.T) {
	var got Alert
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &Notifier{Client: srv.Client(), Guard: noGuard}
	alert := Alert{Event: "scan.changed", Target: "x", NewlyReachable: []string{"http://x/admin"}}
	if err := n.Send(context.Background(), srv.URL, alert); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Target != "x" || len(got.NewlyReachable) != 1 {
		t.Errorf("unexpected payload: %+v", got)
	}
}

func TestSendRejectsNonHTTP(t *testing.T) {
	n := &Notifier{Guard: noGuard}
	if err := n.Send(context.Background(), "ftp://example.com/hook", Alert{}); err == nil {
		t.Error("expected rejection of non-http scheme")
	}
}

func TestSendSSRFGuard(t *testing.T) {
	// Default guard should reject a private/localhost host.
	n := New()
	err := n.Send(context.Background(), "http://localhost:9999/hook", Alert{Target: "x"})
	if err == nil {
		t.Error("expected SSRF guard to reject localhost webhook")
	}
}

func TestSendEmptyURLNoop(t *testing.T) {
	if err := New().Send(context.Background(), "", Alert{}); err != nil {
		t.Errorf("empty url should be a no-op, got %v", err)
	}
}

func TestSendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	n := &Notifier{Client: srv.Client(), Guard: noGuard}
	if err := n.Send(context.Background(), srv.URL, Alert{}); err == nil {
		t.Error("expected error on 5xx webhook response")
	}
}
