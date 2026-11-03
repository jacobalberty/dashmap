package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProbeLivez(t *testing.T) {
	status := http.StatusOK
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/livez" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	if err := probeLivez(u.Host); err != nil {
		t.Fatalf("want healthy, got %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := probeLivez(u.Host); err == nil {
		t.Fatal("want error on 503")
	}
	if err := probeLivez("not-an-addr"); err == nil {
		t.Fatal("want error on bad addr")
	}
}
