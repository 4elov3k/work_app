package models

import "time"

// RedmineProject is the project shape exposed to the frontend.
type RedmineProject struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	IsPublic    bool   `json:"is_public"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}

type RedmineProjectListResponse struct {
	Data    []RedmineProject `json:"data"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	Source  string           `json:"source"`
	Fetched time.Time        `json:"fetched_at"`
}

type CustomerRedmineProjectLink struct {
	ID                 string     `json:"id"`
	CustomerID         string     `json:"customer_id"`
	RedmineProjectID   string     `json:"redmine_project_id"`
	RedmineIdentifier  string     `json:"redmine_identifier"`
	RedmineProjectName string     `json:"redmine_project_name"`
	RedmineURL         string     `json:"redmine_url"`
	LastSyncedAt       *time.Time `json:"last_synced_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CustomerRedmineProjectLinkResponse struct {
	Data *CustomerRedmineProjectLink `json:"data"`
}

type LinkCustomerRedmineProjectRequest struct {
	ProjectID         string `json:"project_id"`
	ProjectIdentifier string `json:"project_identifier"`
	ProjectName       string `json:"project_name"`
	ProjectURL        string `json:"project_url"`
}

type RedmineDocumentStatus struct {
	DocumentType             string     `json:"document_type"`
	DocumentID               string     `json:"document_id"`
	CustomerID               string     `json:"customer_id"`
	RedmineProjectID         string     `json:"redmine_project_id"`
	RedmineProjectIdentifier string     `json:"redmine_project_identifier"`
	RedmineProjectName       string     `json:"redmine_project_name"`
	Filename                 string     `json:"filename"`
	Status                   string     `json:"status"`
	Error                    string     `json:"error"`
	UploadedAt               *time.Time `json:"uploaded_at"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type RedmineDocumentStatusListResponse struct {
	Data []RedmineDocumentStatus `json:"data"`
}

type RedmineIssueAuthor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type RedmineProjectGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RedmineProjectDashboardItem struct {
	ProjectID             string                      `json:"project_id"`
	Identifier            string                      `json:"identifier"`
	Name                  string                      `json:"name"`
	URL                   string                      `json:"url"`
	Description           string                      `json:"description"`
	Status                int                         `json:"status"`
	IsPublic              bool                        `json:"is_public"`
	InferredManagerID     string                      `json:"inferred_manager_id"`
	InferredManagerName   string                      `json:"inferred_manager_name"`
	ManualManagerID       string                      `json:"manual_manager_id"`
	ManualManagerName     string                      `json:"manual_manager_name"`
	EffectiveManagerID    string                      `json:"effective_manager_id"`
	EffectiveManagerName  string                      `json:"effective_manager_name"`
	ProjectType           string                      `json:"project_type"`
	Urgent                bool                        `json:"urgent"`
	UrgentReason          string                      `json:"urgent_reason"`
	DeadlineState         string                      `json:"deadline_state"`
	NextControlEvent      *RedmineProjectControlEvent `json:"next_control_event"`
	InferredIssueID       string                      `json:"inferred_issue_id"`
	InferredAt            *time.Time                  `json:"inferred_at"`
	GroupID               string                      `json:"group_id"`
	GroupName             string                      `json:"group_name"`
	GroupColor            string                      `json:"group_color"`
	GroupAssignedManually bool                        `json:"group_assigned_manually"`
	SyncedAt              time.Time                   `json:"synced_at"`
	CreatedAt             time.Time                   `json:"created_at"`
	UpdatedAt             time.Time                   `json:"updated_at"`
}

// ManagerOption is one entry in the project-dashboard manager picker.
// It carries both ID and name so that two different people who happen to
// share the same Redmine display name are not collapsed into one option.
type ManagerOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RedmineProjectDashboardResponse struct {
	Data      []RedmineProjectDashboardItem `json:"data"`
	Groups    []RedmineProjectGroup         `json:"groups"`
	Managers  []ManagerOption               `json:"managers"`
	SyncedAt  *time.Time                    `json:"synced_at"`
	Refreshed bool                          `json:"refreshed"`
}

type RedmineProjectGroupListResponse struct {
	Data []RedmineProjectGroup `json:"data"`
}

type CreateRedmineProjectGroupRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type UpdateRedmineProjectGroupRequest struct {
	Name     *string `json:"name"`
	Color    *string `json:"color"`
	Position *int    `json:"position"`
}

type UpdateRedmineProjectGroupAssignmentRequest struct {
	GroupID string `json:"group_id"`
}

type UpdateRedmineProjectManagerRequest struct {
	ManagerID   string `json:"manager_id"`
	ManagerName string `json:"manager_name"`
}

type UpdateRedmineProjectOperationsRequest struct {
	ProjectType  *string `json:"project_type"`
	Urgent       *bool   `json:"urgent"`
	UrgentReason *string `json:"urgent_reason"`
}

type RedmineProjectControlEvent struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	EventType      string     `json:"event_type"`
	ServiceType    string     `json:"service_type"`
	Title          string     `json:"title"`
	DueDate        string     `json:"due_date"`
	PeriodStart    string     `json:"period_start"`
	PeriodEnd      string     `json:"period_end"`
	SequenceNumber int        `json:"sequence_number"`
	Status         string     `json:"status"`
	SentAt         *time.Time `json:"sent_at"`
	SentBy         string     `json:"sent_by"`
	RedmineIssueID string     `json:"redmine_issue_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type RedmineProjectControlEventListResponse struct {
	Data []RedmineProjectControlEvent `json:"data"`
}

type GenerateRedmineProjectCycleRequest struct {
	ProjectType string `json:"project_type"`
	ReportDate  string `json:"report_date"`
}

type MarkRedmineProjectControlEventSentRequest struct {
	SentBy string `json:"sent_by"`
}

type RedmineIssue struct {
	ID          int    `json:"id"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Author      string `json:"author"`
	AssignedTo  string `json:"assigned_to"`
	UpdatedOn   string `json:"updated_on"`
	CreatedOn   string `json:"created_on"`
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type RedmineIssueListResponse struct {
	Data  []RedmineIssue `json:"data"`
	Total int            `json:"total"`
}

type RedmineProjectFile struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Filesize    int64  `json:"filesize"`
	ContentType string `json:"content_type"`
	Description string `json:"description"`
	ContentURL  string `json:"content_url"`
	Author      string `json:"author"`
	CreatedOn   string `json:"created_on"`
	Downloads   int    `json:"downloads"`
}

type RedmineProjectFileListResponse struct {
	Data []RedmineProjectFile `json:"data"`
}

type RedmineProjectLocalDocument struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	Number         string  `json:"number"`
	Date           string  `json:"date"`
	Status         string  `json:"status"`
	TotalAmount    float64 `json:"total_amount"`
	ContractNumber string  `json:"contract_number"`
	UploadedStatus string  `json:"uploaded_status"`
	UploadedAt     string  `json:"uploaded_at"`
	URL            string  `json:"url"`
}

type RedmineProjectDocumentsResponse struct {
	CustomerID   string                        `json:"customer_id"`
	CustomerName string                        `json:"customer_name"`
	Files        []RedmineProjectFile          `json:"files"`
	Local        []RedmineProjectLocalDocument `json:"local_documents"`
}

type UploadRedmineDocumentPDFRequest struct {
	DocumentType  string `json:"document_type"`
	DocumentID    string `json:"document_id"`
	Filename      string `json:"filename"`
	ContentBase64 string `json:"content_base64"`
}

type UploadRedmineDocumentPDFResponse struct {
	Status string `json:"status"`
	FileID string `json:"file_id"`
}
