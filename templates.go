package main

import (
	"bytes"
	"html/template"
	"io"
	"log/slog"
	"os"

	"github.com/andrew-d/upchek/internal/buildtags"
	"github.com/andrew-d/upchek/internal/ulog"
)

// registerTemplate returns a function that will return a template. In dev
// mode, the template will be reloaded from disk on every call; in prod, the
// provided embedded template data will be used.
//
// If an error occurs when parsing the template, the returned function will
// log an error and return the embedded template.
//
// If the embedded template does not parse, this function will panic.
func registerTemplate(log *slog.Logger, path string, embedded []byte) func() *template.Template {
	// Parse early so we can panic if the embedded template is
	// invalid.
	t, err := template.New(path).Parse(string(embedded))
	if err != nil {
		log.Error("parsing embedded template", slog.String("path", path), ulog.Error(err))
		panic(err)
	}

	if !buildtags.IsDev {
		log.Debug("production mode, using embedded template")
		return func() *template.Template { return t }
	}

	log.Debug("development mode, reloading template from disk")

	var (
		lastData  []byte
		lastTdisk *template.Template
	)
	return func() *template.Template {
		// Parse the template on every call so we can reload it from disk.
		f, err := os.Open(path)
		if err != nil {
			log.Error("opening template file", slog.String("path", path), ulog.Error(err))
			return t
		}
		defer f.Close()

		tdata, err := io.ReadAll(f)
		if err != nil {
			log.Error("reading template file", slog.String("path", path), ulog.Error(err))
			return t
		}
		if bytes.Equal(tdata, lastData) {
			return lastTdisk
		}

		tdisk, err := template.New(path).Parse(string(tdata))
		if err != nil {
			log.Error("parsing template file", slog.String("path", path), ulog.Error(err))
			return t
		}

		if lastTdisk != nil {
			log.Debug("reloaded template from disk", slog.String("path", path))
		}
		lastData = tdata
		lastTdisk = tdisk
		return tdisk
	}
}
