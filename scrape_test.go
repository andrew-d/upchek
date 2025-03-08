package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/neilotoole/slogt"

	"github.com/andrew-d/upchek/internal/runner"
)

func TestScrape(t *testing.T) {
	// Launch a http server that serves a JSON response.
	fakeNow := time.Unix(1741397010, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sr := []serviceResult{{
			Result: &runner.Result{
				Name:     "foo",
				ExitCode: 0,
				Stdout:   "hello world\n",
			},
			LastRun: fakeNow,
		}}
		json.MarshalWrite(w, sr)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()

	// Create a fetchRemoteResultService.
	s := &service{
		logger:      slogt.New(t),
		remoteAddrs: []string{addr},
	}
	s.initMetrics()
	fr := &fetchRemoteResultService{
		parent:   s,
		addr:     addr,
		interval: 0,
	}

	// A single fetch should succeed.
	ctx := context.Background()
	if err := fr.fetch(ctx, addr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Verify that scraping a page that 500s results in an error.
func TestScrapeError(t *testing.T) {
	// Launch a http server that serves a JSON response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().String()

	// Create a fetchRemoteResultService.
	s := &service{
		logger:      slogt.New(t),
		remoteAddrs: []string{addr},
	}
	s.initMetrics()
	fr := &fetchRemoteResultService{
		parent:   s,
		addr:     addr,
		interval: 0,
	}

	// A single fetch should succeed.
	ctx := context.Background()
	if err := fr.fetch(ctx, addr); err == nil {
		t.Fatalf("expected error")
	}
}
