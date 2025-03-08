// Package lazy provides a lazy evaluation mechanism for Go.
package lazy

type Value[T any] struct {
	value T
	err   error

	// done is whether the value has been computed.
	done bool

	// filling is whether the value is currently being computed.
	filling bool
}

// Set will set the value of the lazy value to v. It will return false if the
// value has already been set.
func (v *Value[T]) Set(val T) bool {
	if v.done {
		return false
	}
	if v.filling {
		panic("Set called while a Get* function is running")
	}
	v.value = val
	v.done = true
	return true
}

// Get returns the value of the lazy value. If the value has not yet been set,
// the fill function will be called to set the value.
func (v *Value[T]) Get(fill func() T) T {
	if !v.done {
		if v.filling {
			panic("Get* function called recursively")
		}

		// Set the "currently being evaluated" flag while we're calling
		// the fill function.
		v.filling = true
		v.value = fill()
		v.filling = false
		v.done = true
	}
	return v.value
}

// GetErr returns the value of the lazy value. If the value has not yet been
// set, the fill function will be called to set the value.
//
// This is the same as the Get function, but for a fill function that returns a
// value and an error.
func (v *Value[T]) GetErr(fill func() (T, error)) (T, error) {
	if !v.done {
		if v.filling {
			panic("Get* function called recursively")
		}

		// Set the "currently being evaluated" flag while we're calling
		// the fill function.
		v.filling = true
		v.value, v.err = fill()
		v.filling = false
		v.done = true
	}
	return v.value, v.err
}
