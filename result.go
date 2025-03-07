package main

import (
	"time"

	"github.com/andrew-d/upchek/internal/runner"
)

// serviceResult wraps the [runner.Result] type with some additional metadata
// that we want to track about results.
type serviceResult struct {
	*runner.Result
	// LastRun is the time the check was last run.
	LastRun time.Time `json:",format:unix"`
}
