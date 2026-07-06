// Package reqctx carries a single mutable record through a request's
// middleware chain so that pieces of information discovered at different
// layers (auth subject, parsed interface/vlan, handler error) can all be
// folded into one request-log row written by the outermost middleware.
package reqctx

import "context"

// Fields is filled in by whichever middleware/handler learns each value.
type Fields struct {
	Subject   string
	Interface string
	VlanID    int
	Error     string
}

type key struct{}

// WithFields attaches a fresh, empty Fields record to ctx and returns both
// the new context and a pointer to the record. Deeper layers that receive
// a context derived from the returned one will see writes made through
// that same pointer.
func WithFields(ctx context.Context) (context.Context, *Fields) {
	f := &Fields{}
	return context.WithValue(ctx, key{}, f), f
}

// From returns the Fields record attached to ctx, or a throwaway record if
// none was attached (so callers never need a nil check).
func From(ctx context.Context) *Fields {
	if f, ok := ctx.Value(key{}).(*Fields); ok {
		return f
	}
	return &Fields{}
}
