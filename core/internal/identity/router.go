package identity

import (
	"github.com/go-chi/chi/v5"
)

type Router struct {
	handler *TenantHandler
}

func NewRouter(handler *TenantHandler) *Router {
	return &Router{
		handler: handler,
	}
}

func (r *Router) Routes() chi.Router {
	mux := chi.NewRouter()
	mux.Post("/", r.handler.CreateTenant)
	return mux
}
