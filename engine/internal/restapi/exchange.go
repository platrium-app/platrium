package restapi

import (
	"context"
	"net/http"
)

type exchangeKey struct{}

type exchange struct {
	w http.ResponseWriter
	r *http.Request
}

// WithExchange is a strict-handler middleware that lets an operation reach its
// raw request and response, for the few that must set a cookie. Operations that
// do not need it never look.
func WithExchange(next StrictHandlerFunc, _ string) StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		return next(context.WithValue(ctx, exchangeKey{}, &exchange{w: w, r: r}), w, r, request)
	}
}

func exchangeFrom(ctx context.Context) (*exchange, bool) {
	ex, ok := ctx.Value(exchangeKey{}).(*exchange)
	return ex, ok
}
