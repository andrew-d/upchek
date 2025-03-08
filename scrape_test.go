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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.MarshalWrite(w, []serviceResult{})
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

	// fetch should fail.
	ctx := context.Background()
	if err := fr.fetch(ctx, addr); err == nil {
		t.Fatalf("expected error")
	} else {
		t.Logf("got expected error: %v", err)
	}

	// Expect an error in the fetch status metric map.
	got := s.metricRemoteFetchStatus.Get(addr)
	if want := "0"; got.String() != want {
		t.Fatalf("expected fetch status for %q to be %q, got %q", addr, want, got)
	}
}
