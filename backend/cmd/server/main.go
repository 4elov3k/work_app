package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"invoices-backend/internal/database"
	"invoices-backend/internal/handlers"
)

func main() {
	// Загрузка конфигурации из переменных окружения
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}

	// Подключение к базе данных
	db, err := database.NewDB(dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Создание handlers
	h := handlers.NewHandlers(db)

	// Настройка роутера
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{corsOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API маршруты
	r.Route("/api", func(r chi.Router) {
		// Customers
		r.Get("/customers", h.GetCustomers)
		r.Post("/customers", h.CreateCustomer)
		r.Get("/customers/{id}", h.GetCustomerByID)

		// Contracts
		r.Get("/contracts", h.GetContracts)
		r.Get("/contracts/next", h.GetNextContractNumber)
		r.Post("/contracts", h.CreateContract)
		r.Get("/contracts/{id}", h.GetContractByID)
		r.Get("/contracts/{id}/next-number", h.GetNextContractDocumentNumber)
		r.Delete("/contracts/{id}", h.DeleteContract)

		// Invoices
		r.Get("/invoices", h.GetInvoices)
		r.Post("/invoices/duplicate", h.DuplicateInvoice)
		r.Get("/invoices/{id}/services", h.GetInvoiceWithServices)
		r.Get("/invoices/{id}", h.GetInvoiceByID)
		r.Post("/invoices", h.CreateInvoice)
		r.Post("/invoices/{id}/lines", h.AddInvoiceLine)
		r.Post("/invoices/{id}/act", h.CreateActFromInvoice)
		r.Patch("/invoices/{id}", h.UpdateInvoice)
		r.Delete("/invoices/{id}", h.DeleteInvoice)

		// Acts
		r.Get("/acts", h.GetActs)
		r.Post("/acts", h.CreateAct)
		r.Get("/acts/{id}/services", h.GetActWithServices)
		r.Get("/acts/{id}/export/upd-xml", h.ExportActUPDXML)
		r.Get("/acts/{id}", h.GetActByID)
		r.Post("/acts/{id}/invoices", h.LinkActInvoices)
		r.Post("/acts/{id}/lines", h.AddActLine)
		r.Patch("/acts/{id}", h.UpdateAct)
		r.Delete("/acts/{id}", h.DeleteAct)

		// Services
		r.Get("/services", h.GetServices)
		r.Post("/services", h.CreateService)
		r.Delete("/services/{id}", h.DeleteService)
	})

	// Запуск сервера
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
