package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"platrium/internal/api"
	"platrium/internal/auth"
	"platrium/internal/auth/actor"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/auth/protocol/oidc"
	"platrium/internal/auth/session"
	"platrium/internal/auth/token"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/graphql"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/kvstore"
	"platrium/internal/infra/storage"
	"platrium/internal/notifications"
	"platrium/internal/notifications/transports"
	"platrium/internal/orchestrator"
	"platrium/internal/restapi"
	"platrium/internal/secrets"
	"platrium/internal/setup"
	"platrium/ui"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/vektah/gqlparser/v2/ast"
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

	database, err := db.NewFromEnv(context.Background())
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer database.Close()
	log.Println("database initialized successfully")

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
	authorizer := sqlauthz.New(database)
	fsOps := fsops.NewFSOps(database, manifestRepo, authorizer)

	// Setup Identity Domain
	tenantStore := identity.NewTenantStore(database)
	groupStore := identity.NewGroupStore(database)
	policyStore := identity.NewPolicyStore(database)
	userStore := identity.NewUserStore(database)
	deviceStore := identity.NewEntDeviceStore(database)

	// Setup Auth Domain
	idpStore := auth.NewIdpStore(database)
	tokenStore := token.NewStore(database, deviceStore, token.DefaultIdleTimeout)
	codeStore := token.NewCodeStore(kvStore)

	// Setup Cross-Domain Orchestrators
	userOrchestrator := orchestrator.NewUserOrchestrator(userStore, fsOps)
	localUserStore := local.NewLocalUserStore(database, userOrchestrator)
	tenantOrchestrator := orchestrator.NewTenantOrchestrator(database, tenantStore, idpStore, localUserStore)

	// Setup Instance Config Store
	instanceConfigStore := setup.NewInstanceConfigStore(kvStore)

	// Setup Setup Orchestrator
	setupOrchestrator := setup.NewOrchestrator(instanceConfigStore, tenantOrchestrator)
	if err := setupOrchestrator.Bootstrap(context.Background()); err != nil {
		log.Fatalf("failed to bootstrap native tenant: %v", err)
	}

	// Setup Session Manager
	sessionManager := session.NewManager()

	// Setup HTTP Routers
	attachedFsHandler := api.NewAttachedFSHandler(storageManager) // we should give it storageManager isntead.

	// Setup Notifications & WebSockets
	gqlTransport := transports.NewGraphQLTransport()
	notifBroker := notifications.NewBroker(gqlTransport)

	actors := actor.NewResolver(authorizer, userStore)
	restAPI := restapi.NewRestAPI(fsOps, actors, chunkStore, storageManager, notifBroker, idpStore, userStore, localUserStore, sessionManager, tokenStore, codeStore, deviceStore)

	// OpenID Connect sign-in
	oidcEndpoints, err := oidc.EndpointsFromEnv()
	if err != nil {
		log.Fatalf("failed to read OIDC endpoint settings: %v", err)
	}
	authManager := auth.NewManager(database, idpStore, userStore, userOrchestrator, sessionManager)
	secretKeys := secrets.FromEnv()
	oidcStore := oidc.NewStore(database, idpStore, secretKeys)
	oidcClient := oidc.NewClient(oidcEndpoints, secretKeys.Sealer(secrets.PurposeAuthFlow))
	oidcHandler := oidc.NewHandler(idpStore, oidcStore, oidcClient, authManager)
	restAPI.OIDC = oidcHandler

	strictHandler := restapi.NewStrictHandler(restAPI, []restapi.StrictMiddlewareFunc{restapi.WithExchange})

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// CORS Settings for Browser Fetch APIs
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://*:3000", "http://*:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "User-Agent", "x-platrium-uploadsession", "x-platrium-downloadsession"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
	}))

	// Setup GraphQL
	gqlResolver := &graphql.Resolver{
		FSOps:       fsOps,
		Authz:       authorizer,
		Actors:      actors,
		DriveOrch:   orchestrator.NewDriveOrchestrator(database, fsOps, authorizer, userStore, policyStore),
		Broker:      notifBroker,
		SubsManager: gqlTransport,
		TenantStore: tenantStore,
		UserStore:   userStore,
		GroupStore:  groupStore,
		IdpStore:    idpStore,
		UserAdmin:   orchestrator.NewUserAdmin(database, userStore, idpStore, localUserStore),
		IdpAdmin:    orchestrator.NewIdpAdmin(userStore, idpStore, oidcStore, oidcClient, oidcEndpoints),
	}
	graphqlSrv := newGraphQLServer(tokenStore, graphql.NewExecutableSchema(graphql.Config{Resolvers: gqlResolver, Directives: gqlResolver.Directives()}))

	graphqlSrv.SetErrorPresenter(graphql.ErrorPresenter)

	// GraphQL Routes
	router.Route("/graphql", func(r chi.Router) {
		r.Use(sessionManager.LoadAndSave)
		r.Use(session.Middleware(sessionManager))
		r.Use(session.Bearer(tokenStore))
		r.Use(actors.Middleware)
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
		r.Use(session.Middleware(sessionManager))
		r.Use(session.Bearer(tokenStore))
		r.Use(actors.Middleware)
		r.Use(rateLimitPath("/api/auth/token", httprate.LimitByIP(20, time.Minute)))
		// Each sign-in start stores a record for a few minutes, and a password
		// login is guessable: bound both per address.
		r.Use(rateLimitPath("/api/auth/login", httprate.LimitByIP(60, time.Minute)))

		r.Get("/health", HealthHandler)
		r.Mount("/attachedfs", attachedFsHandler.Routes())
		r.Get("/auth/oidc/{idpId}/callback", oidcHandler.Callback)

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

// rateLimitPath applies a limiter to a single path and leaves the rest of the
// router alone. The token endpoint is unauthenticated, so it is the one place
// a client can be hammered without credentials.
func rateLimitPath(path string, limiter func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		limited := limiter(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == path {
				limited.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newGraphQLServer is gqlgen's default server plus bearer auth for websocket
// subscriptions. Browsers authenticate the upgrade request with their cookie;
// native clients send {"Authorization": "Bearer ..."} in connection_init.
func newGraphQLServer(tokens *token.Store, es gqlgraphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		InitFunc: func(ctx context.Context, payload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
			secret, ok := session.BearerFromHeader(payload.Authorization())
			if !ok {
				return ctx, &payload, nil
			}
			ctx, err := session.WithBearer(ctx, tokens, secret)
			return ctx, &payload, err
		},
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})
	return srv
}
