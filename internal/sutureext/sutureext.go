// Package suturesext provides a set of extensions to the suture package.
package sutureext

import "context"

// The ServiceFunc type is an adapter to allow the use of ordinary functions as
// a suture.Service. If f is a function with the appropriate signature,
// ServiceFunc(f) is a suture.Service that calls f.
type ServiceFunc func(context.Context) error

func (f ServiceFunc) Serve(ctx context.Context) error { return f(ctx) }
