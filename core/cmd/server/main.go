package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"platrium/internal/api"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"
	"platrium/internal/infra/kvstore"
	"platrium/internal/infra/storage"
	"platrium/internal/objects"
	"platrium/internal/setup"

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
	attachedFSStore := storage.NewAttachedFSStore(store)

	storageProvider := storage.NewStorageProvider(attachedFSStore)
	attachedFS := storage.NewAttachedFSBackend(attachedFSStore)

	manifestRepo := fsops.NewManifestRepo(store)
	fsOps := fsops.NewFSOps(graphStore, manifestRepo)

	// Setup Identity Domain
	tenantStore := identity.NewTenantStore(graphStore)
	userStore := identity.NewUserStore(graphStore)

	// Setup Instance Config Store
	instanceConfigStore := setup.NewInstanceConfigStore(store)

	// Setup Cross-Domain Orchestrator
	setupOrchestrator := setup.NewOrchestrator(instanceConfigStore, tenantStore, userStore, fsOps)
	if err := setupOrchestrator.Bootstrap(context.Background()); err != nil {
		log.Fatalf("failed to bootstrap native tenant: %v", err)
	}

	// Setup HTTP Routers
	fsRouter := fsops.NewRouter(fsOps, storageProvider)
	objectsRouter := objects.NewRouter(storageProvider)
	attachedFsHandler := api.NewAttachedFSHandler(attachedFS)

	identityHandler := identity.NewTenantHandler(tenantStore, userStore, fsOps)
	identityRouter := identity.NewRouter(identityHandler)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Routes
	router.Route("/api", func(r chi.Router) {
		r.Get("/health", HealthHandler)
		r.Mount("/fs", fsRouter.Routes())
		r.Mount("/objects", objectsRouter)
		r.Mount("/attachedfs", attachedFsHandler.Routes())
		r.Mount("/tenants", identityRouter.Routes())
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
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
