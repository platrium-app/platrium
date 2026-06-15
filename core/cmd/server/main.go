package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"platrium/internal/api"
	"platrium/internal/infra/graph"
	"platrium/internal/infra/kvstore"
	"platrium/internal/infra/storage"
	"platrium/internal/repositories"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// @title           Platrium Core API
// @version         1.0.0
// @description     Core API for Platrium artifact management.
// @host            localhost:3000
// @BasePath        /
func main() {
	store, err := kvstore.NewFromEnv()
	if err != nil {
		log.Fatalf("failed to initialize KV store: %v", err)
	}
	defer store.Close()
	log.Println("kv store initialized successfully")

	graphStore, err := graph.NewFromEnv()
	if err != nil {
		log.Fatalf("failed to initialize Graph store: %v", err)
	}
	defer graphStore.Close(context.Background())
	log.Println("graph store initialized successfully")

	// Wire up dependencies
	writesRepo := repositories.NewAttachedFSWritesRepository(store)
	storageProvider := storage.NewStorageProvider(writesRepo)
	attachedFS := storage.NewAttachedFSBackend(writesRepo)

	// fsOps := fsops.NewFSOps(graphStore)

	objectsHandler := api.NewObjectsHandler(storageProvider)
	attachedFsHandler := api.NewAttachedFSHandler(attachedFS)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Routes
	router.Route("/api", func(r chi.Router) {
		r.Get("/health", HealthHandler)
		r.Mount("/objects", objectsHandler.Routes())
		r.Mount("/attachedfs", attachedFsHandler.Routes())
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server listening on :%s", port)
	http.ListenAndServe(":"+port, router)
}

// HealthHandler godoc
// @Summary      Health Check
// @Description  Check if the core is running
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"message": "Platrium Core is running"})
}
