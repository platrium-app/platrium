package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"platrium/internal/api"
	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/auth/session"
	"platrium/internal/fsops"
	"platrium/internal/graphql"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"
	"platrium/internal/infra/kvstore"
	"platrium/internal/infra/storage"
	"platrium/internal/notifications"
	"platrium/internal/notifications/transports"
	"platrium/internal/orchestrator"
	"platrium/internal/restapi"
	"platrium/internal/setup"
	"platrium/ui"

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
	storageManager := storage.NewManager(chunkStore)
	storageManager.StartChunkValidationWorker(context.Background())

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

	// Setup Auth Domain
	idpStore := auth.NewIdpStore(graphStore)
	localUserStore := local.NewLocalUserStore(kvStore)

	// Setup Cross-Domain Orchestrators
	userOrchestrator := orchestrator.NewUserOrchestrator(userStore, fsOps)
	tenantOrchestrator := orchestrator.NewTenantOrchestrator(graphStore, tenantStore, idpStore, userOrchestrator, localUserStore)

	// Setup Instance Config Store
	instanceConfigStore := setup.NewInstanceConfigStore(kvStore)

	// Setup Setup Orchestrator
	setupOrchestrator := setup.NewOrchestrator(instanceConfigStore, tenantOrchestrator)
	if err := setupOrchestrator.Bootstrap(context.Background()); err != nil {
		log.Fatalf("failed to bootstrap native tenant: %v", err)
	}

	// Setup Session Manager & Dev Fallback
	sessionManager := session.NewManager()
	devFallback := &session.PlatriumSession{
		UserID:   "TcnyA2aI3O7YiFXj2OTii",
		TenantID: "pEbCNJV_5Uq9-jcr53YdI",
		Email:    "example@platrium.org",
	}

	// Setup HTTP Routers
	attachedFsHandler := api.NewAttachedFSHandler(storageManager) // we should give it storageManager isntead.

	// Setup Notifications & WebSockets
	gqlTransport := transports.NewGraphQLTransport()
	notifBroker := notifications.NewBroker(gqlTransport)

	restAPI := restapi.NewRestAPI(fsOps, chunkStore, storageManager, notifBroker)
	strictHandler := restapi.NewStrictHandler(restAPI, nil)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// CORS Settings for Browser Fetch APIs
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://*:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "User-Agent", "x-platrium-uploadsession", "x-platrium-downloadsession"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
	}))

	// Setup GraphQL
	graphqlSrv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: &graphql.Resolver{
		FSOps:       fsOps,
		Broker:      notifBroker,
		SubsManager: gqlTransport,
	}}))

	// GraphQL Routes
	router.Route("/graphql", func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(session.Middleware(sessionManager, devFallback))
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Upgrade") == "websocket" {
					r.Header.Del("Origin")
				}
				next.ServeHTTP(w, r)
			})
		})

		r.Handle("/", graphqlSrv)
		r.Handle("/playground", playground.Handler("GraphQL playground", "/graphql"))
	})
	router.Handle("/playground", playground.Handler("GraphQL playground", "/graphql"))

	router.Route("/api", func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(session.Middleware(sessionManager, devFallback))

		r.Get("/health", HealthHandler)
		r.Mount("/attachedfs", attachedFsHandler.Routes())

		// OpenAPI Generated Routes (Strict Server Mode)
		restapi.HandlerFromMux(strictHandler, r)
	})

	// UI Route (Must be the last route registered so it catches all non-API paths)
	router.Handle("/*", ui.Handler())

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
