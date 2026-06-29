package fsops

import (
	"github.com/go-chi/chi/v5"
)

// NewRouter aggregates all domain-specific feature handlers into a single router.
func NewRouter(fsOps *FSOps) chi.Router {
	r := chi.NewRouter()

	fileHandler := NewFileHandler(fsOps)

	r.Post("/createfile", fileHandler.CreateFile)
	
	return r
}
