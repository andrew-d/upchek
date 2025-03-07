package main

import "expvar"

type floatMap struct {
	*expvar.Map
}

func newFloatMap(name string) *floatMap {
	return &floatMap{expvar.NewMap(name)}
}

func (m *floatMap) Set(key string, value float64) {
	// Get or create the expvar.Float for the given key.
	if v := m.Get(key); v != nil {
		v.(*expvar.Float).Set(value)
		return
	}

	fv := new(expvar.Float)
	fv.Set(value)
	m.Map.Set(key, fv)
}

func (m *floatMap) Add(key string, delta float64) {
	// Get or create the expvar.Float for the given key.
	if v := m.Get(key); v != nil {
		v.(*expvar.Float).Add(delta)
		return
	}

	fv := new(expvar.Float)
	fv.Add(delta)
	m.Map.Set(key, fv)
}

type boolMap struct {
	*expvar.Map
}

func newBoolMap(name string) *boolMap {
	return &boolMap{expvar.NewMap(name)}
}

func (m *boolMap) Set(key string, value bool) {
	// Represent a bool as an integer internally.
	var i int64
	if value {
		i = 1
	}

	// Get or create the expvar.Int for the given key.
	if v := m.Get(key); v != nil {
		v.(*expvar.Int).Set(i)
		return
	}

	iv := new(expvar.Int)
	iv.Set(i)
	m.Map.Set(key, iv)
}
