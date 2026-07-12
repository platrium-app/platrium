package fsops

import (
	"github.com/go-chi/chi/v5"

	"platrium/internal/infra/storage"
)

type Router struct {
	handler *FileHandler
}

func NewRouter(fsOps *FSOps, storageProvider *storage.StorageProvider) *Router {
	return &Router{
		handler: NewFileHandler(fsOps, storageProvider),
	}
}

func (r *Router) Routes() chi.Router {
	mux := chi.NewRouter()
	mux.Post("/createfile", r.handler.CreateFile)
	mux.Get("/download/{fileId}", r.handler.DownloadFile)
	return mux
}
