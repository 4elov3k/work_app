package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"invoices-backend/internal/models"
)

// GetRedmineProjects handles GET /api/redmine/projects.
func (h *Handlers) GetRedmineProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	search := r.URL.Query().Get("search")

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			limit = value
		}
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
			offset = value
		}
	}

	projects, total, responseLimit, responseOffset, err := h.redmine.ListProjects(ctx, search, limit, offset)
	if err != nil {
		log.Printf("GetRedmineProjects failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Failed to load Redmine projects")
		return
	}

	respondWithJSON(w, http.StatusOK, models.RedmineProjectListResponse{
		Data:    projects,
		Total:   total,
		Limit:   responseLimit,
		Offset:  responseOffset,
		Source:  h.redmine.BaseURL(),
		Fetched: time.Now(),
	})
}

// GetCustomerRedmineProject handles GET /api/customers/{id}/redmine-project.
func (h *Handlers) GetCustomerRedmineProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	link, err := h.db.GetCustomerRedmineProject(ctx, customerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get customer Redmine project")
		return
	}

	respondWithJSON(w, http.StatusOK, models.CustomerRedmineProjectLinkResponse{Data: link})
}

// LinkCustomerRedmineProject handles PUT /api/customers/{id}/redmine-project.
func (h *Handlers) LinkCustomerRedmineProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	var req models.LinkCustomerRedmineProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ProjectID == "" {
		respondWithError(w, http.StatusBadRequest, "Redmine project ID is required")
		return
	}
	if req.ProjectURL == "" {
		idOrIdentifier := req.ProjectIdentifier
		if idOrIdentifier == "" {
			idOrIdentifier = req.ProjectID
		}
		req.ProjectURL = h.redmine.ProjectURL(idOrIdentifier)
	}

	link, err := h.db.LinkCustomerRedmineProject(ctx, customerID, req)
	if err != nil {
		log.Printf("LinkCustomerRedmineProject failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to link customer Redmine project")
		return
	}

	requestPayload, _ := json.Marshal(req)
	responsePayload, _ := json.Marshal(link)
	if err := h.db.CreateAgentAuditLog(ctx, "operator", "link_customer_redmine_project", "customer", customerID, customerID, "success", "", requestPayload, responsePayload); err != nil {
		log.Printf("failed to write audit log: %v", err)
	}

	respondWithJSON(w, http.StatusOK, models.CustomerRedmineProjectLinkResponse{Data: link})
}

// GetCustomerRedmineDocumentStatuses handles GET /api/customers/{id}/redmine-document-statuses.
func (h *Handlers) GetCustomerRedmineDocumentStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	statuses, err := h.db.GetCustomerRedmineDocumentStatuses(ctx, customerID)
	if err != nil {
		log.Printf("GetCustomerRedmineDocumentStatuses failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get Redmine document statuses")
		return
	}

	respondWithJSON(w, http.StatusOK, models.RedmineDocumentStatusListResponse{Data: statuses})
}

// GetRedmineProjectDashboard handles GET /api/redmine/dashboard.
func (h *Handlers) GetRedmineProjectDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	refresh := r.URL.Query().Get("refresh") == "true" || r.URL.Query().Get("refresh") == "1"

	hasItems, err := h.db.HasRedmineDashboardItems(ctx)
	if err != nil {
		log.Printf("HasRedmineDashboardItems failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to read Redmine dashboard")
		return
	}

	refreshed := false
	if refresh || !hasItems {
		if err := h.syncRedmineProjectDashboard(ctx); err != nil {
			log.Printf("syncRedmineProjectDashboard failed: %v", err)
			respondWithError(w, http.StatusBadGateway, "Failed to sync Redmine project dashboard")
			return
		}
		refreshed = true
	}

	h.respondRedmineDashboard(w, r, refreshed)
}

// SyncRedmineProjectDashboard handles POST /api/redmine/dashboard/sync.
func (h *Handlers) SyncRedmineProjectDashboard(w http.ResponseWriter, r *http.Request) {
	if err := h.syncRedmineProjectDashboard(r.Context()); err != nil {
		log.Printf("syncRedmineProjectDashboard failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Failed to sync Redmine project dashboard")
		return
	}

	h.respondRedmineDashboard(w, r, true)
}

func (h *Handlers) respondRedmineDashboard(w http.ResponseWriter, r *http.Request, refreshed bool) {
	ctx := r.Context()
	items, managers, syncedAt, err := h.db.GetRedmineProjectDashboard(ctx)
	if err != nil {
		log.Printf("GetRedmineProjectDashboard failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to load Redmine dashboard")
		return
	}
	groups, err := h.db.GetRedmineProjectGroups(ctx)
	if err != nil {
		log.Printf("GetRedmineProjectGroups failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to load Redmine project groups")
		return
	}

	respondWithJSON(w, http.StatusOK, models.RedmineProjectDashboardResponse{
		Data:      items,
		Groups:    groups,
		Managers:  managers,
		SyncedAt:  syncedAt,
		Refreshed: refreshed,
	})
}

func (h *Handlers) syncRedmineProjectDashboard(ctx context.Context) error {
	projects, _, _, _, err := h.redmine.ListProjects(ctx, "", 500, 0)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, project := range projects {
		project := project
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			author, issueID, err := h.redmine.LatestIssueAuthor(ctx, project.Identifier)
			if err != nil {
				log.Printf("failed to infer project manager for Redmine project %s: %v", project.Identifier, err)
			}
			if err := h.db.UpsertRedmineProjectDashboardItem(ctx, project, h.redmine.BaseURL(), author, issueID); err != nil {
				log.Printf("failed to upsert Redmine dashboard project %s: %v", project.Identifier, err)
			}
		}()
	}
	wg.Wait()

	requestPayload, _ := json.Marshal(map[string]int{"projects": len(projects)})
	if err := h.db.CreateAgentAuditLog(ctx, "operator", "sync_redmine_project_dashboard", "redmine_project_dashboard", "", "", "success", "", requestPayload, []byte("{}")); err != nil {
		log.Printf("failed to write audit log: %v", err)
	}

	return nil
}

// GetRedmineProjectGroups handles GET /api/redmine/project-groups.
func (h *Handlers) GetRedmineProjectGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.db.GetRedmineProjectGroups(r.Context())
	if err != nil {
		log.Printf("GetRedmineProjectGroups failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to load Redmine project groups")
		return
	}
	respondWithJSON(w, http.StatusOK, models.RedmineProjectGroupListResponse{Data: groups})
}

// CreateRedmineProjectGroup handles POST /api/redmine/project-groups.
func (h *Handlers) CreateRedmineProjectGroup(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRedmineProjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Color = strings.TrimSpace(req.Color)
	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Group name is required")
		return
	}

	group, err := h.db.CreateRedmineProjectGroup(r.Context(), req)
	if err != nil {
		log.Printf("CreateRedmineProjectGroup failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create Redmine project group")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]models.RedmineProjectGroup{"data": *group})
}

// UpdateRedmineProjectGroup handles PATCH /api/redmine/project-groups/{id}.
func (h *Handlers) UpdateRedmineProjectGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "Group ID is required")
		return
	}

	var req models.UpdateRedmineProjectGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name != nil {
		value := strings.TrimSpace(*req.Name)
		req.Name = &value
		if value == "" {
			respondWithError(w, http.StatusBadRequest, "Group name is required")
			return
		}
	}

	group, err := h.db.UpdateRedmineProjectGroup(r.Context(), id, req)
	if err != nil {
		log.Printf("UpdateRedmineProjectGroup failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update Redmine project group")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]models.RedmineProjectGroup{"data": *group})
}

// AssignRedmineProjectGroup handles PUT /api/redmine/dashboard/projects/{projectID}/group.
func (h *Handlers) AssignRedmineProjectGroup(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	var req models.UpdateRedmineProjectGroupAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.db.AssignRedmineProjectGroup(r.Context(), projectID, req.GroupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Redmine project dashboard item not found")
			return
		}
		log.Printf("AssignRedmineProjectGroup failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to assign Redmine project group")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AssignRedmineProjectManager handles PUT /api/redmine/dashboard/projects/{projectID}/manager.
func (h *Handlers) AssignRedmineProjectManager(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	var req models.UpdateRedmineProjectManagerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ManagerID = strings.TrimSpace(req.ManagerID)
	req.ManagerName = strings.TrimSpace(req.ManagerName)

	if err := h.db.AssignRedmineProjectManager(r.Context(), projectID, req.ManagerID, req.ManagerName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Redmine project dashboard item not found")
			return
		}
		log.Printf("AssignRedmineProjectManager failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to assign Redmine project manager")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateRedmineProjectOperations handles PATCH /api/redmine/dashboard/projects/{projectID}.
func (h *Handlers) UpdateRedmineProjectOperations(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	var req models.UpdateRedmineProjectOperationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ProjectType != nil {
		value := normalizeProjectType(*req.ProjectType)
		if value == "" && strings.TrimSpace(*req.ProjectType) != "" {
			respondWithError(w, http.StatusBadRequest, "Invalid project type")
			return
		}
		req.ProjectType = &value
	}
	if req.UrgentReason != nil {
		value := strings.TrimSpace(*req.UrgentReason)
		req.UrgentReason = &value
	}

	if err := h.db.UpdateRedmineProjectOperations(r.Context(), projectID, req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Redmine project dashboard item not found")
			return
		}
		log.Printf("UpdateRedmineProjectOperations failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update Redmine project operations")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetRedmineProjectControlEvents handles GET /api/redmine/dashboard/projects/{projectID}/control-events.
func (h *Handlers) GetRedmineProjectControlEvents(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}
	events, err := h.db.GetRedmineProjectControlEvents(r.Context(), projectID)
	if err != nil {
		log.Printf("GetRedmineProjectControlEvents failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to load Redmine project control events")
		return
	}
	respondWithJSON(w, http.StatusOK, models.RedmineProjectControlEventListResponse{Data: events})
}

// GenerateRedmineProjectCycle handles POST /api/redmine/dashboard/projects/{projectID}/control-events/generate.
func (h *Handlers) GenerateRedmineProjectCycle(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	var req models.GenerateRedmineProjectCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ProjectType = normalizeProjectType(req.ProjectType)
	if req.ProjectType == "" {
		respondWithError(w, http.StatusBadRequest, "Project type is required")
		return
	}
	reportDate, err := time.Parse("2006-01-02", strings.TrimSpace(req.ReportDate))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Report date must be YYYY-MM-DD")
		return
	}

	events, err := h.db.GenerateRedmineProjectCycle(r.Context(), projectID, req.ProjectType, reportDate)
	if err != nil {
		log.Printf("GenerateRedmineProjectCycle failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to generate Redmine project cycle")
		return
	}
	respondWithJSON(w, http.StatusOK, models.RedmineProjectControlEventListResponse{Data: events})
}

// MarkRedmineProjectControlEventSent handles POST /api/redmine/dashboard/projects/{projectID}/control-events/{eventID}/send.
func (h *Handlers) MarkRedmineProjectControlEventSent(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	eventID := chi.URLParam(r, "eventID")
	if projectID == "" || eventID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID and event ID are required")
		return
	}

	var req models.MarkRedmineProjectControlEventSentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.SentBy = strings.TrimSpace(req.SentBy)

	events, err := h.db.MarkRedmineProjectControlEventSent(r.Context(), projectID, eventID, req.SentBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Control event not found")
			return
		}
		log.Printf("MarkRedmineProjectControlEventSent failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to mark control event sent")
		return
	}
	respondWithJSON(w, http.StatusOK, models.RedmineProjectControlEventListResponse{Data: events})
}

// DeleteRedmineProjectControlEvent handles DELETE /api/redmine/dashboard/projects/{projectID}/control-events/{eventID}.
func (h *Handlers) DeleteRedmineProjectControlEvent(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	eventID := chi.URLParam(r, "eventID")
	if projectID == "" || eventID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID and event ID are required")
		return
	}

	events, err := h.db.DeleteRedmineProjectControlEvent(r.Context(), projectID, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Control event not found")
			return
		}
		log.Printf("DeleteRedmineProjectControlEvent failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to delete control event")
		return
	}

	respondWithJSON(w, http.StatusOK, models.RedmineProjectControlEventListResponse{Data: events})
}

func normalizeProjectType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none":
		return ""
	case "seo", "сео":
		return "seo"
	case "ads", "реклама":
		return "ads"
	case "dev", "development", "разработка":
		return "dev"
	case "legal", "юридическая помощь":
		return "legal"
	case "support", "техподдержка":
		return "support"
	default:
		return ""
	}
}

// GetRedmineProjectIssues handles GET /api/redmine/dashboard/projects/{projectID}/issues.
func (h *Handlers) GetRedmineProjectIssues(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	item, err := h.db.GetRedmineProjectDashboardItem(r.Context(), projectID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Redmine project not found")
		return
	}

	issues, total, err := h.redmine.ListOpenIssues(r.Context(), item.ProjectID, 500)
	if err != nil {
		log.Printf("GetRedmineProjectIssues failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Failed to load Redmine project issues")
		return
	}

	respondWithJSON(w, http.StatusOK, models.RedmineIssueListResponse{Data: issues, Total: total})
}

// GetRedmineProjectDocuments handles GET /api/redmine/dashboard/projects/{projectID}/documents.
func (h *Handlers) GetRedmineProjectDocuments(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if projectID == "" {
		respondWithError(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	item, err := h.db.GetRedmineProjectDashboardItem(r.Context(), projectID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Redmine project not found")
		return
	}

	files, err := h.redmine.ListProjectFiles(r.Context(), item.Identifier)
	if err != nil {
		log.Printf("GetRedmineProjectDocuments files failed: %v", err)
		respondWithError(w, http.StatusBadGateway, "Failed to load Redmine project files")
		return
	}

	customerID, customerName, localDocs, err := h.db.GetLocalDocumentsForRedmineProject(r.Context(), item.ProjectID)
	if err != nil {
		log.Printf("GetLocalDocumentsForRedmineProject failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to load local project documents")
		return
	}
	if files == nil {
		files = []models.RedmineProjectFile{}
	}
	if localDocs == nil {
		localDocs = []models.RedmineProjectLocalDocument{}
	}

	respondWithJSON(w, http.StatusOK, models.RedmineProjectDocumentsResponse{
		CustomerID:   customerID,
		CustomerName: customerName,
		Files:        files,
		Local:        localDocs,
	})
}

// UploadRedmineDocumentPDF handles POST /api/redmine/documents/upload-pdf.
func (h *Handlers) UploadRedmineDocumentPDF(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.UploadRedmineDocumentPDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.DocumentType = strings.TrimSpace(req.DocumentType)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.Filename = strings.TrimSpace(req.Filename)

	if req.DocumentType != "invoice" && req.DocumentType != "act" {
		respondWithError(w, http.StatusBadRequest, "Document type must be invoice or act")
		return
	}
	if req.DocumentID == "" || req.Filename == "" || req.ContentBase64 == "" {
		respondWithError(w, http.StatusBadRequest, "Document ID, filename, and PDF content are required")
		return
	}
	if !strings.HasSuffix(strings.ToLower(req.Filename), ".pdf") {
		req.Filename += ".pdf"
	}

	data, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid PDF content")
		return
	}

	customerID := ""
	if req.DocumentType == "invoice" {
		invoice, err := h.db.GetInvoiceByID(ctx, req.DocumentID)
		if err != nil {
			respondWithError(w, http.StatusNotFound, "Invoice not found")
			return
		}
		customerID = invoice.CustomerID
	} else {
		act, err := h.db.GetActByID(ctx, req.DocumentID)
		if err != nil {
			respondWithError(w, http.StatusNotFound, "Act not found")
			return
		}
		customerID = act.CustomerID
	}

	link, err := h.db.GetCustomerRedmineProject(ctx, customerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get customer Redmine project")
		return
	}
	if link == nil {
		respondWithError(w, http.StatusBadRequest, "Customer is not linked to a Redmine project")
		return
	}

	fileID, uploadErr := h.redmine.UploadProjectFile(ctx, link.RedmineIdentifier, req.Filename, "Документ из work-app", data)
	if uploadErr != nil {
		log.Printf("UploadRedmineDocumentPDF failed: %v", uploadErr)
		_ = h.db.RecordRedmineDocumentUpload(ctx, req.DocumentType, req.DocumentID, customerID, link.RedmineProjectID, link.RedmineIdentifier, link.RedmineProjectName, "", req.Filename, "failed", uploadErr.Error())
		respondWithError(w, http.StatusBadGateway, "Failed to upload PDF to Redmine")
		return
	}

	if err := h.db.RecordRedmineDocumentUpload(ctx, req.DocumentType, req.DocumentID, customerID, link.RedmineProjectID, link.RedmineIdentifier, link.RedmineProjectName, fileID, req.Filename, "uploaded", ""); err != nil {
		log.Printf("RecordRedmineDocumentUpload failed: %v", err)
	}

	requestPayload, _ := json.Marshal(map[string]string{
		"document_type": req.DocumentType,
		"document_id":   req.DocumentID,
		"filename":      req.Filename,
		"project_id":    link.RedmineProjectID,
	})
	responsePayload, _ := json.Marshal(map[string]string{"file_id": fileID})
	if err := h.db.CreateAgentAuditLog(ctx, "operator", "upload_document_pdf_to_redmine", req.DocumentType, req.DocumentID, customerID, "success", "", requestPayload, responsePayload); err != nil {
		log.Printf("failed to write audit log: %v", err)
	}

	respondWithJSON(w, http.StatusOK, models.UploadRedmineDocumentPDFResponse{
		Status: "uploaded",
		FileID: fileID,
	})
}
