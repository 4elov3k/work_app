package redmine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"invoices-backend/internal/models"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewFromEnv() *Client {
	baseURL := strings.TrimRight(os.Getenv("REDMINE_URL"), "/")
	apiKey := os.Getenv("REDMINE_API")
	if apiKey == "" {
		apiKey = os.Getenv("REDMINE_API_TOKEN")
	}
	timeout := 60 * time.Second
	if rawTimeout := strings.TrimSpace(os.Getenv("REDMINE_TIMEOUT_SECONDS")); rawTimeout != "" {
		if seconds, err := strconv.Atoi(rawTimeout); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) ProjectURL(identifierOrID string) string {
	if c.baseURL == "" || identifierOrID == "" {
		return ""
	}
	return c.baseURL + "/projects/" + url.PathEscape(identifierOrID)
}

type projectsResponse struct {
	Projects   []models.RedmineProject `json:"projects"`
	TotalCount int                     `json:"total_count"`
	Offset     int                     `json:"offset"`
	Limit      int                     `json:"limit"`
}

type issuesResponse struct {
	Issues []struct {
		ID      int `json:"id"`
		Project struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Subject string `json:"subject"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Author     models.RedmineIssueAuthor `json:"author"`
		AssignedTo struct {
			Name string `json:"name"`
		} `json:"assigned_to"`
		CreatedOn string `json:"created_on"`
		UpdatedOn string `json:"updated_on"`
	} `json:"issues"`
	TotalCount int `json:"total_count"`
}

type filesResponse struct {
	Files []struct {
		ID          int    `json:"id"`
		Filename    string `json:"filename"`
		Filesize    int64  `json:"filesize"`
		ContentType string `json:"content_type"`
		Description string `json:"description"`
		ContentURL  string `json:"content_url"`
		Author      struct {
			Name string `json:"name"`
		} `json:"author"`
		CreatedOn string `json:"created_on"`
		Downloads int    `json:"downloads"`
	} `json:"files"`
}

type uploadResponse struct {
	Upload struct {
		Token string `json:"token"`
	} `json:"upload"`
}

type createProjectFileResponse struct {
	File struct {
		ID int `json:"id"`
	} `json:"file"`
}

func (c *Client) ListProjects(ctx context.Context, search string, limit, offset int) ([]models.RedmineProject, int, int, int, error) {
	if !c.Configured() {
		return nil, 0, 0, 0, fmt.Errorf("redmine is not configured")
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	var projects []models.RedmineProject
	total := 0
	nextOffset := offset
	pageSize := 100
	for len(projects) < limit {
		remaining := limit - len(projects)
		if remaining < pageSize {
			pageSize = remaining
		}

		page, err := c.listProjectsPage(ctx, pageSize, nextOffset)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		total = page.TotalCount
		projects = append(projects, page.Projects...)

		if len(page.Projects) == 0 || len(projects) >= total {
			break
		}
		nextOffset += page.Limit
	}

	if search != "" {
		needle := strings.ToLower(search)
		filtered := projects[:0]
		for _, project := range projects {
			if strings.Contains(strings.ToLower(project.Name), needle) ||
				strings.Contains(strings.ToLower(project.Identifier), needle) {
				filtered = append(filtered, project)
			}
		}
		projects = filtered
	}

	return projects, total, limit, offset, nil
}

func (c *Client) listProjectsPage(ctx context.Context, limit, offset int) (*projectsResponse, error) {
	endpoint, err := url.Parse(c.baseURL + "/projects.json")
	if err != nil {
		return nil, fmt.Errorf("invalid redmine url: %w", err)
	}

	query := endpoint.Query()
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build redmine request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request redmine projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("redmine projects request failed with HTTP %d", resp.StatusCode)
	}

	var parsed projectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to parse redmine projects: %w", err)
	}

	return &parsed, nil
}

func (c *Client) LatestIssueAuthor(ctx context.Context, projectIdentifier string) (*models.RedmineIssueAuthor, string, error) {
	if !c.Configured() {
		return nil, "", fmt.Errorf("redmine is not configured")
	}
	if projectIdentifier == "" {
		return nil, "", fmt.Errorf("project identifier is required")
	}

	endpoint, err := url.Parse(c.baseURL + "/issues.json")
	if err != nil {
		return nil, "", fmt.Errorf("invalid redmine url: %w", err)
	}

	query := endpoint.Query()
	query.Set("project_id", projectIdentifier)
	query.Set("limit", "1")
	query.Set("sort", "created_on:desc")
	query.Set("status_id", "*")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build redmine issues request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to request redmine issues: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("redmine issues request failed with HTTP %d", resp.StatusCode)
	}

	var parsed issuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", fmt.Errorf("failed to parse redmine issues: %w", err)
	}
	if len(parsed.Issues) == 0 {
		return nil, "", nil
	}

	issue := parsed.Issues[0]
	return &issue.Author, strconv.Itoa(issue.ID), nil
}

func (c *Client) ListOpenIssues(ctx context.Context, projectID string, limit int) ([]models.RedmineIssue, int, error) {
	if !c.Configured() {
		return nil, 0, fmt.Errorf("redmine is not configured")
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 500 {
		limit = 500
	}

	var issues []models.RedmineIssue
	total := 0
	offset := 0
	for len(issues) < limit {
		pageSize := 100
		if remaining := limit - len(issues); remaining < pageSize {
			pageSize = remaining
		}

		pageIssues, pageTotal, err := c.listOpenIssuesPage(ctx, projectID, pageSize, offset)
		if err != nil {
			return nil, 0, err
		}
		total = pageTotal
		issues = append(issues, pageIssues...)
		if len(pageIssues) == 0 || len(issues) >= total {
			break
		}
		offset += pageSize
	}

	return issues, total, nil
}

func (c *Client) listOpenIssuesPage(ctx context.Context, projectID string, limit, offset int) ([]models.RedmineIssue, int, error) {
	endpoint, err := url.Parse(c.baseURL + "/issues.json")
	if err != nil {
		return nil, 0, fmt.Errorf("invalid redmine url: %w", err)
	}

	query := endpoint.Query()
	query.Set("project_id", projectID)
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))
	query.Set("sort", "updated_on:desc")
	query.Set("status_id", "open")
	endpoint.RawQuery = query.Encode()

	var parsed issuesResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint.String(), nil, "application/json", &parsed); err != nil {
		return nil, 0, err
	}

	issues := make([]models.RedmineIssue, 0, len(parsed.Issues))
	for _, issue := range parsed.Issues {
		issues = append(issues, models.RedmineIssue{
			ID:          issue.ID,
			Subject:     issue.Subject,
			Status:      issue.Status.Name,
			Priority:    issue.Priority.Name,
			Author:      issue.Author.Name,
			AssignedTo:  issue.AssignedTo.Name,
			UpdatedOn:   issue.UpdatedOn,
			CreatedOn:   issue.CreatedOn,
			ProjectID:   issue.Project.ID,
			ProjectName: issue.Project.Name,
		})
	}

	return issues, parsed.TotalCount, nil
}

func (c *Client) ListProjectFiles(ctx context.Context, projectID string) ([]models.RedmineProjectFile, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("redmine is not configured")
	}

	endpoint, err := url.Parse(c.baseURL + "/projects/" + url.PathEscape(projectID) + "/files.json")
	if err != nil {
		return nil, fmt.Errorf("invalid redmine url: %w", err)
	}

	var parsed filesResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint.String(), nil, "application/json", &parsed); err != nil {
		return nil, err
	}

	files := make([]models.RedmineProjectFile, 0, len(parsed.Files))
	for _, file := range parsed.Files {
		files = append(files, models.RedmineProjectFile{
			ID:          file.ID,
			Filename:    file.Filename,
			Filesize:    file.Filesize,
			ContentType: file.ContentType,
			Description: file.Description,
			ContentURL:  file.ContentURL,
			Author:      file.Author.Name,
			CreatedOn:   file.CreatedOn,
			Downloads:   file.Downloads,
		})
	}

	return files, nil
}

func (c *Client) UploadProjectFile(ctx context.Context, projectID, filename, description string, data []byte) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("redmine is not configured")
	}
	if projectID == "" {
		return "", fmt.Errorf("project id is required")
	}
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}

	if files, err := c.ListProjectFiles(ctx, projectID); err == nil {
		for _, file := range files {
			if file.Filename == filename {
				return strconv.Itoa(file.ID), nil
			}
		}
	}

	uploadURL := c.baseURL + "/uploads.json?filename=" + url.QueryEscape(filename)
	var upload uploadResponse
	if err := c.doJSON(ctx, http.MethodPost, uploadURL, bytes.NewReader(data), "application/octet-stream", &upload); err != nil {
		return "", err
	}
	if upload.Upload.Token == "" {
		return "", fmt.Errorf("redmine upload did not return token")
	}

	body := map[string]map[string]string{
		"file": {
			"token":       upload.Upload.Token,
			"filename":    filename,
			"description": description,
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to encode redmine file payload: %w", err)
	}

	createURL := c.baseURL + "/projects/" + url.PathEscape(projectID) + "/files.json"
	var created createProjectFileResponse
	if err := c.doJSON(ctx, http.MethodPost, createURL, bytes.NewReader(encoded), "application/json", &created); err != nil {
		return "", err
	}
	if created.File.ID == 0 {
		files, err := c.ListProjectFiles(ctx, projectID)
		if err != nil {
			return "", nil
		}
		for _, file := range files {
			if file.Filename == filename {
				return strconv.Itoa(file.ID), nil
			}
		}
		return "", nil
	}
	return strconv.Itoa(created.File.ID), nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, body io.Reader, contentType string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to build redmine request: %w", err)
	}
	req.Header.Set("X-Redmine-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request redmine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("redmine request failed with HTTP %d", resp.StatusCode)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("failed to parse redmine response: %w", err)
	}
	return nil
}
