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
	"platrium/internal/restapi"
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
	kvStore, err := kvstore.NewFromEnv()
	if err != nil {
		log.Fatalf("failed to initialize KV store: %v", err)
	}
	defer kvStore.Close()
	log.Println("kv store initialized successfully")

	graphStore, err := graph.NewFromEnv()
	if err != nil {
		log.Fatalf("failed to initialize Graph store: %v", err)
	}
	defer graphStore.Close(context.Background())
	log.Println("graph store initialized successfully")

	// Initialize Chunk Store
	chunkStore := fsops.NewChunkStore(kvStore)

	// Setup Attached FS
	attachedfsStore := storage.NewAttachedFSStore(kvStore)

	// Setup Storage Manager
	storageManager := storage.NewManager()
	storageManager.StartChunkValidationWorker(context.Background(), chunkStore)

	storageManager.RegisterBackendType("attachedfs", storage.AttachedFSBackendFactory(attachedfsStore))

	manifestRepo := fsops.NewManifestRepo(kvStore)
	fsOps := fsops.NewFSOps(graphStore, manifestRepo)

	// Setup Identity Domain
	tenantStore := identity.NewTenantStore(graphStore)
	userStore := identity.NewUserStore(graphStore)

	// Setup Instance Config Store
	instanceConfigStore := setup.NewInstanceConfigStore(kvStore)

	// Setup Cross-Domain Orchestrator
	setupOrchestrator := setup.NewOrchestrator(instanceConfigStore, tenantStore, userStore, fsOps)
	if err := setupOrchestrator.Bootstrap(context.Background()); err != nil {
		log.Fatalf("failed to bootstrap native tenant: %v", err)
	}

	// Setup HTTP Routers
	objectsRouter := objects.NewRouter(storageManager)
	attachedFsHandler := api.NewAttachedFSHandler(storageManager) // we should give it storageManager isntead.

	identityHandler := identity.NewTenantHandler(tenantStore, userStore, fsOps)
	identityRouter := identity.NewRouter(identityHandler)

	restAPI := restapi.NewRestAPI(fsOps, chunkStore, storageManager)
	strictHandler := restapi.NewStrictHandler(restAPI, nil)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Routes
	router.Route("/api", func(r chi.Router) {
		r.Get("/health", HealthHandler)
		r.Mount("/objects", objectsRouter)
		r.Mount("/attachedfs", attachedFsHandler.Routes())
		r.Mount("/tenants", identityRouter.Routes())

		// OpenAPI Generated Routes (Strict Server Mode)
		restapi.HandlerFromMux(strictHandler, r)
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
	json.NewEncoder(w).Encode(map[string]string{"message": "Platrium Engine is running"})
}
