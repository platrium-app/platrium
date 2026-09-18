package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"platrium/internal/api"
	"platrium/internal/auth/session"
	"platrium/internal/fsops"
	"platrium/internal/graphql"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"
	"platrium/internal/infra/kvstore"
	"platrium/internal/infra/storage"
	"platrium/internal/objects"
	"platrium/internal/restapi"
	"platrium/internal/setup"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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
	storageManager.StartBackend(context.Background(), "default", storage.BackendConfig{
		Type:   "attachedfs",
		Config: json.RawMessage(`{"mount_path":"./data"}`),
	})

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

	// Setup Session Manager & Dev Fallback
	sessionManager := session.NewManager()
	devFallback := &session.PlatriumSession{
		UserID:   "99dff953-bdf8-40b9-859d-897c363455da",
		TenantID: "c6038c5b-6a37-4c52-ad3c-1fa3c631fe1a",
		Email:    "admin@example.com",
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

	// CORS Settings for Browser Fetch APIs
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173", "http://*:5173"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "User-Agent", "x-platrium-uploadsession"},
		ExposedHeaders: []string{"Link"},
	}))

	// Setup GraphQL
	graphqlSrv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: &graphql.Resolver{
		FSOps: fsOps,
	}}))

	// GraphQL Routes
	router.Route("/graphql", func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(session.Middleware(sessionManager, devFallback))
		r.Handle("/", graphqlSrv)
		r.Handle("/playground", playground.Handler("GraphQL playground", "/graphql"))
	})
	router.Handle("/playground", playground.Handler("GraphQL playground", "/graphql"))

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
