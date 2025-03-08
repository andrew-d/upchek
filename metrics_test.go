package main

import (
	"expvar"
	"testing"
)

func TestFloatMap_Set(t *testing.T) {
	// NOTE: construct manually instead of with newFloatMap since we can
	// only register a given expvar.Map once.
	m := newFloatMap()

	m.Set("key", 1.0)
	if v := m.Get("key"); v == nil {
		t.Fatal("expected value to be set")
	} else if v.(*expvar.Float).Value() != 1.0 {
		t.Fatalf("expected value to be 1.0, got %v", v.(*expvar.Float).Value())
	}
}

func TestFloatMap_Add(t *testing.T) {
	m := newFloatMap()

	m.Add("key", 1.0)
	if v := m.Get("key"); v == nil {
		t.Fatal("expected value to be set")
	} else if v.(*expvar.Float).Value() != 1.0 {
		t.Fatalf("expected value to be 1.0, got %v", v.(*expvar.Float).Value())
	}

	m.Add("key", 1.0)
	if v := m.Get("key"); v == nil {
		t.Fatal("expected value to be set")
	} else if v.(*expvar.Float).Value() != 2.0 {
		t.Fatalf("expected value to be 2.0, got %v", v.(*expvar.Float).Value())
	}
}

func TestBoolMap(t *testing.T) {
	m := newBoolMap()

	m.Set("key", true)
	if v := m.Get("key"); v == nil {
		t.Fatal("expected value to be set")
	} else if v.(*expvar.Int).Value() != 1 {
		t.Fatalf("expected value to be 1, got %v", v.(*expvar.Int).Value())
	}

	m.Set("key", false)
	if v := m.Get("key"); v == nil {
		t.Fatal("expected value to be set")
	} else if v.(*expvar.Int).Value() != 0 {
		t.Fatalf("expected value to be 0, got %v", v.(*expvar.Int).Value())
	}
}
