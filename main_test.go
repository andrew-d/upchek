package main

import (
	"fmt"
	"testing"

	"github.com/andrew-d/upchek/internal/runner"
)

func TestIndexData(t *testing.T) {
	successResult := serviceResult{
		Result: &runner.Result{
			Name:     "good.sh",
			ExitCode: 0,
			Stdout:   "success\n",
		},
	}
	errorResult := serviceResult{
		Result: &runner.Result{
			Name:     "bad.sh",
			ExitCode: 1,
			Stdout:   "error\n",
		},
	}

	t.Run("LocalOk", func(t *testing.T) {
		t.Run("Ok", func(t *testing.T) {
			data := indexData{
				Results: []serviceResult{successResult},
			}
			if !data.LocalOk() {
				t.Error("LocalOk() should return true")
			}
			if !data.GlobalOk() {
				t.Error("GlobalOk() should return true")
			}
		})
		t.Run("Fail", func(t *testing.T) {
			data := indexData{
				Results: []serviceResult{errorResult},
			}
			if data.LocalOk() {
				t.Error("LocalOk() should return false")
			}
			if data.GlobalOk() {
				t.Error("GlobalOk() should return false")
			}
		})
	})
	t.Run("RemoteOk", func(t *testing.T) {
		t.Run("Ok", func(t *testing.T) {
			data := indexData{
				RemoteAddrs: []string{"localhost:0"},
				RemoteResults: map[string][]serviceResult{
					"localhost:0": {successResult},
				},
			}
			if !data.RemoteOk() {
				t.Error("RemoteOk() should return true")
			}
			if !data.GlobalOk() {
				t.Error("GlobalOk() should return true")
			}
		})
		t.Run("Fail", func(t *testing.T) {
			data := indexData{
				RemoteAddrs: []string{"localhost:0"},
				RemoteResults: map[string][]serviceResult{
					"localhost:0": {errorResult},
				},
			}
			if data.RemoteOk() {
				t.Error("RemoteOk() should return false")
			}
			if data.GlobalOk() {
				t.Error("GlobalOk() should return false")
			}
		})
		t.Run("Error", func(t *testing.T) {
			data := indexData{
				RemoteAddrs: []string{"localhost:0"},
				RemoteErrors: map[string]error{
					"localhost:0": fmt.Errorf("error"),
				},
			}
			if data.RemoteOk() {
				t.Error("RemoteOk() should return false")
			}
			if data.GlobalOk() {
				t.Error("GlobalOk() should return false")
			}
		})
	})
}
