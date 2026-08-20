// Redmine-синхронизация: проекты, дашборд менеджеров, контрольные срезы/отчётные даты.
import { API_BASE, handleResponse, SingleResponse } from './client';

export interface RedmineProject {
  id: number;
  name: string;
  identifier: string;
  description: string;
  status: number;
  is_public: boolean;
  created_on: string;
  updated_on: string;
}

export interface RedmineProjectLink {
  id: string;
  customer_id: string;
  redmine_project_id: string;
  redmine_identifier: string;
  redmine_project_name: string;
  redmine_url: string;
  last_synced_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface RedmineDocumentStatus {
  document_type: "invoice" | "act";
  document_id: string;
  customer_id: string;
  redmine_project_id: string;
  redmine_project_identifier: string;
  redmine_project_name: string;
  filename: string;
  status: "pending" | "uploaded" | "failed";
  error: string;
  uploaded_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface RedmineProjectGroup {
  id: string;
  name: string;
  color: string;
  position: number;
  created_at: string;
  updated_at: string;
}

export type RedmineProjectType = "" | "seo" | "ads" | "dev" | "legal" | "support";
export type RedmineDeadlineState = "ok" | "due_soon" | "burning" | "urgent";

export interface RedmineProjectControlEvent {
  id: string;
  project_id: string;
  event_type: "report_date" | "control_cut";
  service_type: RedmineProjectType;
  title: string;
  due_date: string;
  period_start: string;
  period_end: string;
  sequence_number: number;
  status: "planned" | "sent" | "skipped";
  sent_at: string | null;
  sent_by: string;
  redmine_issue_id: string;
  created_at: string;
  updated_at: string;
}

export interface RedmineProjectDashboardItem {
  project_id: string;
  identifier: string;
  name: string;
  url: string;
  description: string;
  status: number;
  is_public: boolean;
  inferred_manager_id: string;
  inferred_manager_name: string;
  manual_manager_id: string;
  manual_manager_name: string;
  effective_manager_id: string;
  effective_manager_name: string;
  project_type: RedmineProjectType;
  urgent: boolean;
  urgent_reason: string;
  deadline_state: RedmineDeadlineState;
  next_control_event: RedmineProjectControlEvent | null;
  inferred_issue_id: string;
  inferred_at: string | null;
  group_id: string;
  group_name: string;
  group_color: string;
  group_assigned_manually: boolean;
  synced_at: string;
  created_at: string;
  updated_at: string;
}

export interface RedmineProjectDashboardResponse {
  data: RedmineProjectDashboardItem[];
  groups: RedmineProjectGroup[];
  managers: string[];
  synced_at: string | null;
  refreshed: boolean;
}

export interface RedmineIssue {
  id: number;
  subject: string;
  status: string;
  priority: string;
  author: string;
  assigned_to: string;
  updated_on: string;
  created_on: string;
  project_id: number;
  project_name: string;
}

export interface RedmineProjectFile {
  id: number;
  filename: string;
  filesize: number;
  content_type: string;
  description: string;
  content_url: string;
  author: string;
  created_on: string;
  downloads: number;
}

export interface RedmineProjectLocalDocument {
  id: string;
  type: "invoice" | "act";
  number: string;
  date: string;
  status: string;
  total_amount: number;
  contract_number: string;
  uploaded_status: string;
  uploaded_at: string;
  url: string;
}

export interface RedmineProjectDocumentsResponse {
  customer_id: string;
  customer_name: string;
  files: RedmineProjectFile[];
  local_documents: RedmineProjectLocalDocument[];
}

export const redmineAPI = {
  getProjects: async (search = "", limit = 100, offset = 0): Promise<{
    data: RedmineProject[];
    total: number;
    limit: number;
    offset: number;
    source: string;
    fetched_at: string;
  }> => {
    const params = new URLSearchParams({
      limit: String(limit),
      offset: String(offset),
    });
    if (search.trim()) params.set("search", search.trim());
    const response = await fetch(`${API_BASE}/redmine/projects?${params.toString()}`);
    return handleResponse<{
      data: RedmineProject[];
      total: number;
      limit: number;
      offset: number;
      source: string;
      fetched_at: string;
    }>(response);
  },

  getDashboard: async (refresh = false): Promise<RedmineProjectDashboardResponse> => {
    const params = new URLSearchParams();
    if (refresh) params.set("refresh", "true");
    const suffix = params.toString() ? `?${params.toString()}` : "";
    const response = await fetch(`${API_BASE}/redmine/dashboard${suffix}`);
    return handleResponse<RedmineProjectDashboardResponse>(response);
  },

  syncDashboard: async (): Promise<RedmineProjectDashboardResponse> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/sync`, {
      method: "POST",
    });
    return handleResponse<RedmineProjectDashboardResponse>(response);
  },

  createProjectGroup: async (data: { name: string; color?: string }): Promise<SingleResponse<RedmineProjectGroup>> => {
    const response = await fetch(`${API_BASE}/redmine/project-groups`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<RedmineProjectGroup>>(response);
  },

  updateProjectGroup: async (
    id: string,
    data: { name?: string; color?: string; position?: number }
  ): Promise<SingleResponse<RedmineProjectGroup>> => {
    const response = await fetch(`${API_BASE}/redmine/project-groups/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<RedmineProjectGroup>>(response);
  },

  assignProjectGroup: async (projectId: string, groupId: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/group`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ group_id: groupId }),
    });
    return handleResponse<{ status: string }>(response);
  },

  assignProjectManager: async (
    projectId: string,
    data: { manager_id?: string; manager_name: string }
  ): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/manager`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<{ status: string }>(response);
  },

  updateProjectOperations: async (
    projectId: string,
    data: { project_type?: RedmineProjectType; urgent?: boolean; urgent_reason?: string }
  ): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<{ status: string }>(response);
  },

  getProjectControlEvents: async (projectId: string): Promise<{ data: RedmineProjectControlEvent[] }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/control-events`);
    return handleResponse<{ data: RedmineProjectControlEvent[] }>(response);
  },

  generateProjectCycle: async (
    projectId: string,
    data: { project_type: RedmineProjectType; report_date: string }
  ): Promise<{ data: RedmineProjectControlEvent[] }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/control-events/generate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<{ data: RedmineProjectControlEvent[] }>(response);
  },

  markControlEventSent: async (
    projectId: string,
    eventId: string,
    data: { sent_by?: string } = {}
  ): Promise<{ data: RedmineProjectControlEvent[] }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/control-events/${eventId}/send`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<{ data: RedmineProjectControlEvent[] }>(response);
  },

  deleteControlEvent: async (
    projectId: string,
    eventId: string
  ): Promise<{ data: RedmineProjectControlEvent[] }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/control-events/${eventId}`, {
      method: "DELETE",
    });
    return handleResponse<{ data: RedmineProjectControlEvent[] }>(response);
  },

  getProjectIssues: async (projectId: string): Promise<{ data: RedmineIssue[]; total: number }> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/issues`);
    return handleResponse<{ data: RedmineIssue[]; total: number }>(response);
  },

  getProjectDocuments: async (projectId: string): Promise<RedmineProjectDocumentsResponse> => {
    const response = await fetch(`${API_BASE}/redmine/dashboard/projects/${projectId}/documents`);
    return handleResponse<RedmineProjectDocumentsResponse>(response);
  },

  uploadDocumentPdf: async (data: {
    document_type: "invoice" | "act";
    document_id: string;
    filename: string;
    content_base64: string;
  }): Promise<{ status: string; file_id: string }> => {
    const response = await fetch(`${API_BASE}/redmine/files/upload-pdf`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return handleResponse<{ status: string; file_id: string }>(response);
  },
};
