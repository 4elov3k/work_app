package main

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"invoices-backend/internal/accounting"
	"invoices-backend/internal/database"
	"invoices-backend/internal/mcpserver"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := database.NewDB(dbURL)
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}
	defer db.Close()

	service := accounting.NewService(db, accounting.Config{
		DocumentStoragePath: getenv("DOCUMENT_STORAGE_PATH", "/data/documents"),
		TokenTTL:            getenvDuration("CONFIRMATION_TOKEN_TTL", 15*time.Minute),
		AllowFinalization:   getenvBool("ALLOW_DOCUMENT_FINALIZATION", true),
		AllowSending:        getenvBool("ALLOW_DOCUMENT_SENDING", false),
	})
	server := mcpserver.New(service)

	switch strings.ToLower(getenv("MCP_TRANSPORT", "http")) {
	case "stdio":
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	case "http", "streamable_http":
		startHTTP(server, service)
	default:
		log.Fatalf("unsupported MCP_TRANSPORT %q", os.Getenv("MCP_TRANSPORT"))
	}
}

func startHTTP(server *mcp.Server, service *accounting.Service) {
	host := getenv("MCP_HOST", "0.0.0.0")
	port := getenv("MCP_PORT", "3000")
	path := getenv("MCP_PATH", "/mcp")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := service.CurrentOrganization(r.Context()); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse:   true,
		SessionTimeout: 30 * time.Minute,
	})
	mux.Handle(path, authMiddleware(mcpHandler))

	addr := host + ":" + port
	log.Printf("Accounting MCP server listening on %s%s", addr, path)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func authMiddleware(next http.Handler) http.Handler {
	enabled := getenvBool("MCP_AUTH_ENABLED", true)
	token := os.Getenv("MCP_AUTH_TOKEN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enabled {
			if token == "" {
				http.Error(w, "MCP_AUTH_TOKEN is required", http.StatusServiceUnavailable)
				return
			}
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		userID := strings.TrimSpace(r.Header.Get("X-Hermes-User"))
		next.ServeHTTP(w, r.WithContext(accounting.WithUser(r.Context(), userID)))
	})
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
