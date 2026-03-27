package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"platrium/ent"
	"platrium/internal/artifacts"
	"platrium/internal/encoding"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
)

// @title           Platrium Core API
// @version         1.0.0
// @description     Core API for Platrium artifact management.
// @host            localhost:3000
// @BasePath        /
func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	client, err := ent.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed opening connection: %v", err)
	}
	defer client.Close()

	if err := client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema: %v", err)
	}
	log.Println("database migration successful")

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Swagger UI route
	// router.Get("/swagger/*", httpSwagger.WrapHandler)

	// Routes
	router.Get("/health", HealthHandler)
	router.Put("/uploads/{hash}", UploadHandler)
	router.Get("/uploads/{hash}", TestGetChunk)

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

// UploadHandler godoc
// @Summary      Upload binary data
// @Description  Streams binary data directly to the storage layer
// @Accept       octet-stream
// @Param        hash  path      string  true  "File Hash"
// @Success      204
// @Failure      500   {string}  string "Internal Server Error"
// @Router       /uploads/{hash} [put]
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	defer r.Body.Close()

	opts := encoding.BlobEncodingOptions{
		Encrypt: false, // Encryption disabled for now as requested
	}

	up, err := artifacts.NewUploadCacheArtifact(hash, r.Body, opts)
	if err != nil {
		http.Error(w, "invalid hash or encoding error: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := up.Save(); err != nil {
		http.Error(w, "storage save error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func TestGetChunk(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	defer r.Body.Close()

	up, err := artifacts.GetUploadCacheArtifact(hash)
	if err != nil {
		http.Error(w, "storage load error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)

	// io.Copy streams the io.Reader natively to the network in chunks (usually 32KB).
	// This uses almost 0 memory regardless of file size!
	_, err = io.Copy(w, up.Source)
	if err != nil {
		log.Printf("error streaming artifact: %v", err)
	}
}
