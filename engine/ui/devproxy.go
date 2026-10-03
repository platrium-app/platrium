//go:build !embed_ui

package ui

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// Handler serves the UI by reverse-proxying to the Vite dev server.
// This file is only compiled when the 'embed_ui' build tag is NOT provided.
func Handler() http.Handler {
	proxyURLStr := os.Getenv("DEV_UI_PROXY")
	if proxyURLStr == "" {
		proxyURLStr = "http://localhost:5173"
	}

	viteURL, err := url.Parse(proxyURLStr)
	if err != nil {
		log.Fatalf("failed to parse DEV_UI_PROXY URL: %v", err)
	}
	
	log.Println("UI: Running in DEV mode, proxying to", viteURL)
	return httputil.NewSingleHostReverseProxy(viteURL)
}
