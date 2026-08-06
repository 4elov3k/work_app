package main

import (
	"log"
	"net/http"
	"os"
	"strings"

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

	corsOrigins := strings.Split(os.Getenv("CORS_ORIGIN"), ",")
	if len(corsOrigins) == 1 && strings.TrimSpace(corsOrigins[0]) == "" {
		corsOrigins = []string{"http://localhost:3000"}
	}
	for index := range corsOrigins {
		corsOrigins[index] = strings.TrimSpace(corsOrigins[index])
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
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API маршруты
	r.Route("/api", func(r chi.Router) {
		// Organization
		r.Get("/organization", h.GetOrganization)

		// Customers
		r.Get("/customers", h.GetCustomers)
		r.Post("/customers", h.CreateCustomer)
		r.Get("/customers/lookup", h.LookupCustomerByINN)
		r.Get("/customers/{id}", h.GetCustomerByID)
		r.Delete("/customers/{id}", h.DeleteCustomer)
		r.Get("/customers/{id}/redmine-project", h.GetCustomerRedmineProject)
		r.Put("/customers/{id}/redmine-project", h.LinkCustomerRedmineProject)
		r.Get("/customers/{id}/redmine-document-statuses", h.GetCustomerRedmineDocumentStatuses)

		// Redmine
		r.Get("/redmine/projects", h.GetRedmineProjects)
		r.Get("/redmine/dashboard", h.GetRedmineProjectDashboard)
		r.Post("/redmine/dashboard/sync", h.SyncRedmineProjectDashboard)
		r.Get("/redmine/project-groups", h.GetRedmineProjectGroups)
		r.Post("/redmine/project-groups", h.CreateRedmineProjectGroup)
		r.Patch("/redmine/project-groups/{id}", h.UpdateRedmineProjectGroup)
		r.Patch("/redmine/dashboard/projects/{projectID}", h.UpdateRedmineProjectOperations)
		r.Put("/redmine/dashboard/projects/{projectID}/group", h.AssignRedmineProjectGroup)
		r.Put("/redmine/dashboard/projects/{projectID}/manager", h.AssignRedmineProjectManager)
		r.Get("/redmine/dashboard/projects/{projectID}/control-events", h.GetRedmineProjectControlEvents)
		r.Post("/redmine/dashboard/projects/{projectID}/control-events/generate", h.GenerateRedmineProjectCycle)
		r.Post("/redmine/dashboard/projects/{projectID}/control-events/{eventID}/send", h.MarkRedmineProjectControlEventSent)
		r.Delete("/redmine/dashboard/projects/{projectID}/control-events/{eventID}", h.DeleteRedmineProjectControlEvent)
		r.Get("/redmine/dashboard/projects/{projectID}/issues", h.GetRedmineProjectIssues)
		r.Get("/redmine/dashboard/projects/{projectID}/documents", h.GetRedmineProjectDocuments)
		r.Post("/redmine/files/upload-pdf", h.UploadRedmineDocumentPDF)
		r.Post("/redmine/documents/upload-pdf", h.UploadRedmineDocumentPDF)

		// Contracts
		r.Get("/contracts", h.GetContracts)
		r.Get("/contracts/next", h.GetNextContractNumber)
		r.Post("/contracts", h.CreateContract)
		r.Get("/contracts/{id}", h.GetContractByID)
		r.Get("/contracts/{id}/next-number", h.GetNextContractDocumentNumber)
		r.Delete("/contracts/{id}", h.DeleteContract)

		// Contract appendices (приложения к договору)
		r.Get("/contracts/{id}/appendices", h.GetContractAppendices)
		r.Post("/contracts/{id}/appendices", h.CreateContractAppendix)
		r.Get("/contracts/{id}/appendices/next-number", h.GetNextContractAppendixNumber)
		r.Get("/contract-appendices/{id}", h.GetContractAppendix)
		r.Patch("/contract-appendices/{id}", h.UpdateContractAppendix)
		r.Delete("/contract-appendices/{id}", h.DeleteContractAppendix)
		r.Post("/contract-appendices/{id}/lines", h.AddContractAppendixLine)
		r.Delete("/contract-appendices/{id}/lines/{lineID}", h.DeleteContractAppendixLine)

		// Invoices
		r.Get("/invoices", h.GetInvoices)
		r.Post("/invoices/duplicate", h.DuplicateInvoice)
		r.Get("/invoices/{id}/services", h.GetInvoiceWithServices)
		r.Get("/invoices/{id}/export/upd-xml", h.ExportInvoiceXML)
		r.Get("/invoices/{id}", h.GetInvoiceByID)
		r.Post("/invoices", h.CreateInvoice)
		r.Post("/invoices/{id}/lines", h.AddInvoiceLine)
		r.Patch("/invoices/{id}/lines/{lineID}", h.UpdateInvoiceLine)
		r.Delete("/invoices/{id}/lines/{lineID}", h.DeleteInvoiceLine)
		r.Post("/invoices/{id}/act", h.CreateActFromInvoice)
		r.Patch("/invoices/{id}", h.UpdateInvoice)
		r.Delete("/invoices/{id}", h.DeleteInvoice)

		// Acts
		r.Get("/acts", h.GetActs)
		r.Post("/acts", h.CreateAct)
		r.Get("/acts/next-number-from-sheet", h.GetActNumberFromSheet)
		r.Post("/acts/{id}/register-in-sheet", h.RegisterActInSheet)
		r.Get("/acts/{id}/services", h.GetActWithServices)
		r.Get("/acts/{id}/export/upd-xml", h.ExportActUPDXML)
		r.Get("/acts/{id}", h.GetActByID)
		r.Post("/acts/{id}/invoices", h.LinkActInvoices)
		r.Post("/acts/{id}/lines", h.AddActLine)
		r.Patch("/acts/{id}/lines/{lineID}", h.UpdateActLine)
		r.Delete("/acts/{id}/lines/{lineID}", h.DeleteActLine)
		r.Patch("/acts/{id}", h.UpdateAct)
		r.Delete("/acts/{id}", h.DeleteAct)

		// Documents
		r.Post("/documents/parse-contract", h.ParseContractDocument)

		// Services
		r.Get("/services/catalog", h.GetServiceCatalog)
		r.Get("/services", h.GetServices)
		r.Post("/services", h.CreateService)
		r.Delete("/services/{id}", h.DeleteService)

		// Звонари (OnlinePBX CDR + Hermes transcribe/analytics)
		r.Get("/zvonari/callers", h.GetCallers)
		r.Post("/zvonari/sync", h.SyncZvonariCalls)
		r.Get("/zvonari/sync/status", h.GetZvonariSyncStatus)
		r.Post("/zvonari/calls/retry-failed", h.RetryFailedCalls)
		r.Get("/zvonari/calls/status-counts", h.GetCallStatusCounts)
		r.Get("/zvonari/calls/count", h.GetCallCounts)
		r.Get("/zvonari/callers/{id}/calls", h.GetCallerCalls)
		r.Post("/zvonari/calls/{id}/transcribe", h.RetranscribeCall)
		r.Get("/zvonari/callers/{id}/distribution", h.GetCallerCallDistribution)
		r.Post("/zvonari/callers/{id}/report", h.RequestCallerReport)
	})

	// Запуск сервера
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
