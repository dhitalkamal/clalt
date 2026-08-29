package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchProfileTopLevelEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth = %q", got)
		}
		_, _ = w.Write([]byte(`{"email":"top@x.com"}`))
	}))
	defer srv.Close()
	p, err := FetchProfile(context.Background(), srv.Client(), srv.URL, "tok")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.Email != "top@x.com" {
		t.Errorf("email = %q", p.Email)
	}
}

func TestFetchProfileNestedEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"account":{"email":"nested@x.com"},"organization":{"name":"Acme"}}`))
	}))
	defer srv.Close()
	p, _ := FetchProfile(context.Background(), srv.Client(), srv.URL, "tok")
	if p.Email != "nested@x.com" {
		t.Errorf("email = %q", p.Email)
	}
	if p.Organization != "Acme" {
		t.Errorf("org = %q", p.Organization)
	}
}

func TestFetchProfileErrorIsNonFatalSignal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	// caller ignores the error, but it should be returned (not panic / not empty success)
	_, err := FetchProfile(context.Background(), srv.Client(), srv.URL, "tok")
	if err == nil {
		t.Error("expected error on 404")
	}
}
