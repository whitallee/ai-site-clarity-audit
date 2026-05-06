package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"ai-site-clarity-audit/auditor"
	"ai-site-clarity-audit/scraper"
)

var (
	auditStore   = make(map[string]*auditor.AuditResult)
	auditStoreMu sync.RWMutex
)

func main() {
	_ = godotenv.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/audit", handleAudit)
	mux.HandleFunc("GET /api/audit/{id}/pdf", handlePDF)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleAudit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.URL, "http") {
		req.URL = "https://" + req.URL
	}

	scraped, err := scraper.Scrape(req.URL)
	if err != nil {
		http.Error(w, "scrape failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	result, err := auditor.Audit(r.Context(), scraped)
	if err != nil {
		http.Error(w, "audit failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	auditStoreMu.Lock()
	auditStore[id] = result
	auditStoreMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "audit": result})
}

func handlePDF(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	auditStoreMu.RLock()
	result, ok := auditStore[id]
	auditStoreMu.RUnlock()

	if !ok {
		http.Error(w, "audit not found", http.StatusNotFound)
		return
	}

	pdfBytes, err := auditor.RenderPDF(context.Background(), result)
	if err != nil {
		http.Error(w, "pdf generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="site-audit-%s.pdf"`, id[:8]))
	w.Write(pdfBytes)
}
