package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-json-experiment/json"

	"github.com/andrew-d/upchek/internal/ulog"
)

type fetchRemoteResultService struct {
	parent   *service
	addr     string
	interval time.Duration
}

func (fr *fetchRemoteResultService) Serve(ctx context.Context) error {
	fr.parent.initMetrics()

	// Fetch once immediately.
	if err := fr.fetch(ctx, fr.addr); err != nil {
		return fmt.Errorf("initial fetch: %w", err)
	}

	ticker := time.NewTicker(fr.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := fr.fetch(ctx, fr.addr); err != nil {
				fr.parent.logger.Error("failed to fetch remote result", ulog.Error(err))
			}
		}
	}
}

func (fr *fetchRemoteResultService) String() string {
	return fmt.Sprintf("fetchRemoteResultService(%s)", fr.addr)
}

func (fr *fetchRemoteResultService) fetch(ctx context.Context, addr string) (retErr error) {
	// Store errors from this fetch
	s := fr.parent
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.remoteErrors == nil {
			s.remoteErrors = make(map[string]error)
		}
		s.remoteErrors[addr] = retErr
	}()

	// Make a request to the remote instance's JSON endpoint.
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/api/v1/results", addr), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Unmarshal into a slice of serviceResult.
	var results []serviceResult
	if err := json.UnmarshalRead(resp.Body, &results); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	// Update metrics
	s.metricRemoteLatency.Set(addr, float64(time.Since(t0).Seconds()))

	ok := true
	for _, result := range results {
		if !result.IsSuccess() {
			ok = false
			break
		}
	}
	s.metricRemoteStatus.Set(addr, ok)

	s.logger.Debug("fetched remote results",
		slog.String("addr", addr),
		slog.Duration("duration", time.Since(t0)),
		slog.Int("count", len(results)),
	)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.remoteResults == nil {
		s.remoteResults = make(map[string][]serviceResult)
	}
	s.remoteResults[addr] = results
	return nil
}
