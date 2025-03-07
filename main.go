package main

import (
	"bytes"
	"context"
	"errors"
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

	"github.com/spf13/pflag"
	"github.com/thejerf/suture/v4"
	"github.com/thejerf/sutureslog"

	"github.com/andrew-d/upchek/internal/buildtags"
	"github.com/andrew-d/upchek/internal/runner"
	"github.com/andrew-d/upchek/internal/suturehttp"
	"github.com/andrew-d/upchek/internal/ulog"
)

var (
	flagVerbose = pflag.BoolP("verbose", "v", false, "verbose output")
	flagListen  = pflag.StringP("listen", "l", ":8080", "address to listen on (e.g., :8080 or 127.0.0.1:8080)")
	flagDir     = pflag.StringP("directory", "d", defaultDir(), "directory for healthcheck scripts")
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
		dir:    *flagDir,
		logger: logger.With(ulog.Component("runner")),
	}
	supervisor.Add(service)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", service.handleIndex)
	mux.HandleFunc("GET /healthz", service.handleHealthz)

	// Add the listener service
	server := suturehttp.New(ln, mux)
	server.Logger = logger.With(ulog.Component("http"))
	supervisor.Add(server)

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

	mu      sync.RWMutex
	results []*runner.Result
}

func (s *service) Serve(ctx context.Context) error {
	if s.logger == nil {
		s.logger = slog.Default()
	}
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

		// Run it
		t0 := time.Now()
		result, err := runner.Run(ctx, fullPath)
		s.logger.Debug("ran script", slog.String("name", entry.Name()), slog.Duration("duration", time.Since(t0)))
		if err != nil {
			return fmt.Errorf("running script: %w", err)
		}

		// Save the result.
		s.results = append(s.results, result)
	}
	return nil
}

func isExecutable(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Mode().Perm()&0111 != 0
}

var indexTemplate = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html>
<head>
<title>upchek</title>

<!-- monospace everywhere! -->
<style>
body {
  font-family: monospace;
}

table {
  border-collapse: collapse;
  width: 100%;
}

th, td {
  border: 1px solid black;
  padding: 8px;
}
</style>

</head>

<body>
<h1>upchek</h1>

<table>
  <thead>
    <tr>
      <th>Script</th>
      <th>Exit Code</th>
      <th>Output</th>
      <th>Error</th>
    </tr>
  </thead>
  <tbody>
  {{range .}}
  <tr>
    <td>{{.Name}}</td>
    <td {{if .IsSuccess}}style="background-color: green"{{else}}style="background-color: red"{{end}}>
      {{.ExitCode}}
    </td>
    <td><pre>{{.Stdout}}</pre></td>
    <td><pre>{{.Stderr}}</pre></td>
  </tr>
  {{end}}
</table>
</body>
</html>
`))

func (s *service) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html")
	indexTemplate.Execute(w, s.results)
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
