package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"expvar"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/spf13/pflag"
	"github.com/thejerf/suture/v4"
	"github.com/thejerf/sutureslog"

	"github.com/andrew-d/upchek/internal/buildtags"
	"github.com/andrew-d/upchek/internal/lazy"
	"github.com/andrew-d/upchek/internal/runner"
	"github.com/andrew-d/upchek/internal/suturehttp"
	"github.com/andrew-d/upchek/internal/ulog"
)

var (
	flagVerbose = pflag.BoolP("verbose", "v", false, "verbose output")
	flagListen  = pflag.StringP("listen", "l", ":8080", "address to listen on (e.g., :8080 or 127.0.0.1:8080)")
	flagDir     = pflag.StringP("directory", "d", defaultDir(), "directory for healthcheck scripts")
	flagRemote  = pflag.StringArray("remote", nil, "list of other upchek instances to aggregate results from")
)

func defaultDir() string {
	if buildtags.IsDev {
		// Run from the project root in dev mode.
		currentDir, err := os.Getwd()
		if err == nil {
			return filepath.Join(currentDir, "scripts")
		}
	}
	return "/etc/upchek"
}

// Templates
var (
	//go:embed index.html.tmpl
	embeddedIndex []byte
)

func main() {
	pflag.Parse()

	// We're using slog for logging.
	logger := slog.Default()
	if *flagVerbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	if buildtags.IsDev {
		logger = logger.With(slog.Bool("dev", true))
	}

	// Listen on provided address.
	ln, err := net.Listen("tcp", *flagListen)
	if err != nil {
		logger.Error("failed to listen", ulog.Error(err))
		os.Exit(1)
	}
	defer ln.Close()

	supervisor := suture.New("upchek", suture.Spec{
		EventHook: (&sutureslog.Handler{Logger: logger}).MustHook(),
	})

	// Set up healthcheck service
	service := &service{
		dir:           *flagDir,
		logger:        logger.With(ulog.Component("runner")),
		indexTemplate: registerTemplate(logger, "index.html.tmpl", embeddedIndex),
		remoteAddrs:   *flagRemote,
	}
	supervisor.Add(service)

	// If we have remote addresses to proxy from, set up a suture service
	// for each of them.
	//
	// TODO: should this be in a separate supervisor with more specific
	// timeouts?
	for _, addr := range service.remoteAddrs {
		supervisor.Add(&fetchRemoteResultService{
			parent:   service,
			addr:     addr,
			interval: 30 * time.Second,
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", service.handleIndex)
	mux.HandleFunc("GET /api/v1/results", service.handleResultsAPI)
	mux.HandleFunc("GET /healthz", service.handleHealthz)
	mux.Handle("/debug/vars", expvar.Handler())

	// Add the listener service
	server := suturehttp.New(ln, mux)
	server.Logger = logger.With(ulog.Component("http"))
	supervisor.Add(server)

	// Publish metrics from our service to expvar; only call once at the
	// top level to avoid duplicate metric panics.
	service.PublishMetrics()

	// Now that we've set up our supervision tree, we can start it.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errc := supervisor.ServeBackground(ctx)
	logger.Info("supervisor started", slog.String("addr", ln.Addr().String()))
	err = <-errc
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("supervisor exited with error", ulog.Error(err))
		os.Exit(1)
	}

	logger.Info("supervisor exited cleanly")
}

type service struct {
	logger *slog.Logger
	dir    string

	// templates
	indexTemplate func() *template.Template

	// metrics
	metricOnce              sync.Once
	metricScriptLatency     *floatMap
	metricScriptSuccess     *boolMap // map[string]bool
	metricLastRun           *expvar.Int
	metricRemoteLatency     *floatMap
	metricRemoteFetchStatus *boolMap // whether we can fetch from a remote
	metricRemoteStatus      *boolMap // aggregate across all results of a remote

	// remote instances
	remoteAddrs []string

	mu            sync.RWMutex // protects following
	results       []serviceResult
	remoteResults map[string][]serviceResult // map[addr][]serviceResult
	remoteErrors  map[string]error           // map[addr]error
}

func (s *service) Serve(ctx context.Context) error {
	if s.logger == nil {
		s.logger = slog.Default()
	}
	s.initMetrics()

	s.logger.Info("runner started", slog.String("dir", s.dir))
	defer s.logger.Info("runner stopped")

	// Run scripts immediately on startup.
	if err := s.runScripts(ctx); err != nil {
		return fmt.Errorf("initial run: %w", err)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := s.runScripts(ctx); err != nil {
				s.logger.Error("failed to run scripts", ulog.Error(err))
			}
		}
	}
	return nil
}

func (s *service) initMetrics() {
	s.metricOnce.Do(func() {
		s.metricScriptLatency = newFloatMap()
		s.metricScriptSuccess = newBoolMap()
		s.metricLastRun = new(expvar.Int)
		s.metricRemoteLatency = newFloatMap()
		s.metricRemoteFetchStatus = newBoolMap()
		s.metricRemoteStatus = newBoolMap()
	})
}

func (s *service) PublishMetrics() {
	s.initMetrics()

	const metricsPrefix = "upchek_"
	expvar.Publish(metricsPrefix+"script_latency", s.metricScriptLatency)
	expvar.Publish(metricsPrefix+"script_last_status", s.metricScriptSuccess)
	expvar.Publish(metricsPrefix+"last_run", s.metricLastRun)
	expvar.Publish(metricsPrefix+"remote_latency", s.metricRemoteLatency)
	expvar.Publish(metricsPrefix+"remote_fetch_status", s.metricRemoteFetchStatus)
	expvar.Publish(metricsPrefix+"remote_status", s.metricRemoteStatus)
}

func (s *service) runScripts(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Start by listing all scripts in the directory.
	dir, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	s.results = s.results[:0]
	for _, entry := range dir {
		// Only run executable files.
		fullPath := filepath.Join(s.dir, entry.Name())
		if !isExecutable(fullPath) {
			s.logger.Debug("skipping non-executable file", slog.String("name", entry.Name()))
			continue
		}

		result, err := s.runScript(ctx, entry.Name(), fullPath)
		if err != nil {
			return fmt.Errorf("running script: %w", err)
		}
		s.results = append(s.results, result)
	}

	s.metricLastRun.Set(time.Now().Unix())
	return nil
}

func (s *service) runScript(ctx context.Context, name, path string) (serviceResult, error) {
	t0 := time.Now()
	result, err := runner.Run(ctx, path)

	// Track metrics before we return.
	s.metricScriptLatency.Set(name, float64(time.Since(t0).Seconds()))
	s.metricScriptSuccess.Set(name, result.IsSuccess())

	s.logger.Debug("ran script", slog.String("name", name), slog.Duration("duration", time.Since(t0)))

	if err != nil {
		return serviceResult{}, err
	}
	return serviceResult{
		Result:  result,
		LastRun: t0,
	}, nil
}

func isExecutable(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Mode().Perm()&0111 != 0
}

func (s *service) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := s.getTemplateData()
	//s.logger.Debug("rendering index", slog.Any("data", data))

	w.Header().Set("Content-Type", "text/html")
	err := s.indexTemplate().Execute(w, data)
	if err != nil {
		s.logger.Error("failed to render index", ulog.Error(err))
	}
}

type indexData struct {
	// Local results
	Results []serviceResult

	// Remote results
	RemoteAddrs   []string
	RemoteResults map[string][]serviceResult
	RemoteErrors  map[string]error

	// Map of remote addresses to status
	lazyRemoteStatus lazy.Value[map[string]bool]

	// Boolean status
	lazyGlobalOk lazy.Value[bool] // all checks, local and remote
	lazyLocalOk  lazy.Value[bool] // local checks only
	lazyRemoteOk lazy.Value[bool] // remote checks only
}

func (d *indexData) GlobalOk() bool {
	return d.lazyGlobalOk.Get(func() bool {
		return d.LocalOk() && d.RemoteOk()
	})
}

func (d *indexData) LocalOk() bool {
	return d.lazyLocalOk.Get(func() bool {
		for _, result := range d.Results {
			if !result.IsSuccess() {
				return false
			}
		}
		return true
	})
}

func (d *indexData) RemoteOk() bool {
	return d.lazyRemoteOk.Get(func() bool {
		for _, ok := range d.RemoteStatus() {
			if !ok {
				return false
			}
		}
		return true
	})
}

// RemoteStatus returns a map with one key per remote address, and a boolean
// value indicating whether all checks from that remote are successful and the
// remote could be scraped successfully..
func (d *indexData) RemoteStatus() map[string]bool {
	return d.lazyRemoteStatus.Get(func() map[string]bool {
		status := make(map[string]bool)
		for _, addr := range d.RemoteAddrs {
			status[addr] = true

			for _, result := range d.RemoteResults[addr] {
				if !result.IsSuccess() {
					status[addr] = false
					break
				}
			}
			if err := d.RemoteErrors[addr]; err != nil {
				status[addr] = false
			}
		}
		return status
	})
}

func (s *service) getTemplateData() (data *indexData) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &indexData{
		Results:       s.results,
		RemoteAddrs:   s.remoteAddrs,
		RemoteResults: s.remoteResults,
		RemoteErrors:  s.remoteErrors,
	}
}

func (s *service) handleResultsAPI(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(s.results)
	if err != nil {
		http.Error(w, "failed to marshal results", http.StatusInternalServerError)
		return
	}

	if buildtags.IsDev {
		(*jsontext.Value)(&b).Indent() // indent for readability
	}
	w.Write(b)
}

func (s *service) handleHealthz(w http.ResponseWriter, r *http.Request) {
	isVerbose := r.URL.Query().Has("verbose")

	w.Header().Set("Content-Type", "text/plain")

	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		ok   bool = true
		body bytes.Buffer
	)
	for _, result := range s.results {
		if isVerbose {
			if result.IsSuccess() {
				fmt.Fprintf(&body, "[+]%s ok\n", result.Name)
			} else {
				fmt.Fprintf(&body, "[-]%s failed\n", result.Name)
			}
		}

		if !result.IsSuccess() {
			ok = false
		}
	}

	if ok {
		w.WriteHeader(http.StatusOK)
		body.WriteString("ok\n")
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		body.WriteString("unhealthy\n")
	}
	io.Copy(w, &body)
}
