package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequestIDMiddlewareInjectsHeader(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequestID)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(r)
	defer srv.Close()

	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("X-Request-Id") == "" {
		t.Fatal("X-Request-Id not set")
	}
}

func TestRecoverMiddlewareReturns500OnPanic(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Recover(NewNopLogger()))
	r.Get("/", func(_ http.ResponseWriter, _ *http.Request) { panic("boom") })
	srv := httptest.NewServer(r)
	defer srv.Close()

	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
}
