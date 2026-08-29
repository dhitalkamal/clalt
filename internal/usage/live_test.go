package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLive(t *testing.T) {
	var gotAuth, gotBeta, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBeta = r.Header.Get("anthropic-beta")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{
			"five_hour": {"utilization": 0.12, "resets_at": "2026-08-24T15:00:00Z"},
			"seven_day": {"utilization": 0.55, "resets_at": "2026-08-27T05:00:00Z"}
		}`))
	}))
	defer srv.Close()

	lu, err := FetchLive(context.Background(), srv.Client(), srv.URL, "sk-ant-oat01-x")
	if err != nil {
		t.Fatalf("FetchLive: %v", err)
	}
	if gotPath != "/api/oauth/usage" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer sk-ant-oat01-x" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotBeta == "" {
		t.Error("anthropic-beta header missing")
	}
	if lu.FiveHour.Utilization != 0.12 || lu.SevenDay.Utilization != 0.55 {
		t.Errorf("utilization parse wrong: %+v", lu)
	}
	if lu.FiveHour.ResetsAt.IsZero() {
		t.Error("resets_at not parsed")
	}
}

func TestFetchLiveRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
	}))
	defer srv.Close()
	_, err := FetchLive(context.Background(), srv.Client(), srv.URL, "tok")
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !IsRateLimited(err) {
		t.Errorf("IsRateLimited() should be true for a 429, got err=%v", err)
	}
}
