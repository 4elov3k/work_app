// API Base URL
const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://127.0.0.1:8080/api';

// Типы данных
export interface Customer {
  id: string;
  name: string;
  fullname: string;
  address: string;
  inn: string;
  kpp: string;
  phone?: string;
  email?: string;
  contact_person?: string;
  contact_position?: string;
  comment?: string;
  status?: string;
  created_at: string;
  updated_at: string;
}

export interface CustomerLookup {
  name: string;
  fullname: string;
  address: string;
  inn: string;
  kpp: string;
  type: string;
  status: string;
  contact_person?: string;
  contact_position?: string;
}

export interface Contract {
  id: string;
  customer_id: string;
  number: string;
  currency: string;
  status: string;
  topic: string;
  start_date: string;
  end_date: string;
  created_at: string;
  updated_at: string;
}

export interface Invoice {
  id: string;
  contract_id: string;
  customer_id: string;
  number: string;
  date: string;
  status: string;
  total_amount: number;
  archived: boolean;
  contract_number: string;
  created_at: string;
  updated_at: string;
}

export interface Act {
  id: string;
  contract_id: string;
  customer_id: string;
  number: string;
  date: string;
  status: string;
  total_amount: number;
  archived: boolean;
  contract_number: string;
  created_at: string;
  updated_at: string;
}

export interface Service {
  id: string;
  name: string;
  unit?: string;
  price: number;
  qty?: number;
  amount?: number;
  section?: string;
  price_per_hour?: number;
  hours_per_unit?: number;
  archived?: boolean;
  created_at: string;
  updated_at: string;
}

export interface ServiceCatalogSection {
  section: string;
  items: Service[];
}

export interface ContractAppendixLine {
  id: string;
  appendix_id: string;
  service_id?: string;
  section: string;
  position: number;
  title: string;
  unit: string;
  price: number;
  qty: number;
  amount: number;
}

export interface ContractAppendix {
  id: string;
  contract_id: string;
  number: string;
  date: string;
  status: string;
  total_amount: number;
  archived: boolean;
  created_at: string;
  updated_at: string;
}

export interface ContractAppendixWithLines extends ContractAppendix {
  lines: ContractAppendixLine[];
}

export interface ContractAppendixLineInput {
  service_id?: string;
  section?: string;
  title?: string;
  unit?: string;
  price?: number;
  qty?: number;
}

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
  event_type: "report_date" | "control_cut" | "roadmap_milestone";
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

export interface InvoiceWithServices extends Invoice {
  services: Service[];
}

export interface ActWithServices extends Act {
  services: Service[];
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface SingleResponse<T> {
  data: T;
}

export interface NullableSingleResponse<T> {
  data: T | null;
}

export interface ErrorResponse {
  error: string;
  code: number;
}

// Ошибка API-запроса с сохранённым HTTP-статусом, чтобы вызывающий код мог
// различать «не найдено» (404) и настоящие сбои сервера/сети.
export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

// Утилита для обработки ответов
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const rawText = await response.text().catch(() => '');
    let message = response.statusText || 'Request failed';
    try {
      const parsed = rawText ? JSON.parse(rawText) : null;
      if (parsed && typeof parsed === 'object') {
        const maybeError = (parsed as Partial<ErrorResponse> & { message?: string }).error;
        const maybeMessage = (parsed as { message?: string }).message;
        message = (maybeError || maybeMessage || message) as string;
      } else if (rawText) {
        message = rawText;
      }
    } catch {
      if (rawText) message = rawText;
    }
    throw new ApiError(`${message} (HTTP ${response.status})`, response.status);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const contentLength = response.headers.get('content-length');
  if (contentLength === '0') {
    return undefined as T;
  }
  const text = await response.text();
  if (!text) {
    return undefined as T;
  }
  return JSON.parse(text) as T;
}

// API для работы с контрагентами
export const customersAPI = {
  // Получить список всех контрагентов
  getAll: async (page = 1, perPage = 100): Promise<PaginatedResponse<Customer>> => {
    const response = await fetch(`${API_BASE}/customers?page=${page}&per_page=${perPage}`);
    return handleResponse<PaginatedResponse<Customer>>(response);
  },

  // Поиск контрагентов
  search: async (query: string, page = 1, perPage = 100): Promise<PaginatedResponse<Customer>> => {
    const response = await fetch(
      `${API_BASE}/customers?search=${encodeURIComponent(query)}&page=${page}&per_page=${perPage}`
    );
    return handleResponse<PaginatedResponse<Customer>>(response);
  },

  // Получить контрагента по ID
  getById: async (id: string): Promise<SingleResponse<Customer>> => {
    const response = await fetch(`${API_BASE}/customers/${id}`);
    return handleResponse<SingleResponse<Customer>>(response);
  },

  // Проверить и подтянуть реквизиты по ИНН
  lookupByInn: async (inn: string, kpp = ""): Promise<SingleResponse<CustomerLookup>> => {
    const params = new URLSearchParams({ inn });
    if (kpp) params.set("kpp", kpp);
    const response = await fetch(`${API_BASE}/customers/lookup?${params.toString()}`);
    return handleResponse<SingleResponse<CustomerLookup>>(response);
  },

  // Создать контрагента
  create: async (data: {
    name: string;
    fullname: string;
    address: string;
    inn: string;
    kpp?: string;
    phone?: string;
    email?: string;
    contact_person?: string;
    contact_position?: string;
    comment?: string;
  }): Promise<SingleResponse<Customer>> => {
    const response = await fetch(`${API_BASE}/customers`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Customer>>(response);
  },

  getRedmineProject: async (id: string): Promise<NullableSingleResponse<RedmineProjectLink>> => {
    const response = await fetch(`${API_BASE}/customers/${id}/redmine-project`);
    return handleResponse<NullableSingleResponse<RedmineProjectLink>>(response);
  },

  linkRedmineProject: async (id: string, data: {
    project_id: string;
    project_identifier?: string;
    project_name?: string;
    project_url?: string;
  }): Promise<SingleResponse<RedmineProjectLink>> => {
    const response = await fetch(`${API_BASE}/customers/${id}/redmine-project`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<RedmineProjectLink>>(response);
  },

  getRedmineDocumentStatuses: async (id: string): Promise<{ data: RedmineDocumentStatus[] }> => {
    const response = await fetch(`${API_BASE}/customers/${id}/redmine-document-statuses`);
    return handleResponse<{ data: RedmineDocumentStatus[] }>(response);
  },
};

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

// API для работы с договорами
export const contractsAPI = {
  getByCustomer: async (customerId: string, page = 1, perPage = 100): Promise<PaginatedResponse<Contract>> => {
    const url = `${API_BASE}/contracts?customer_id=${customerId}&page=${page}&per_page=${perPage}`;
    const response = await fetch(url);
    return handleResponse<PaginatedResponse<Contract>>(response);
  },

  getById: async (id: string): Promise<SingleResponse<Contract>> => {
    const response = await fetch(`${API_BASE}/contracts/${id}`);
    return handleResponse<SingleResponse<Contract>>(response);
  },

  create: async (data: {
    customer_id: string;
    number: string;
    currency?: string;
    status?: string;
    topic: string;
    start_date?: string;
    end_date?: string;
  }): Promise<SingleResponse<Contract>> => {
    const response = await fetch(`${API_BASE}/contracts`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Contract>>(response);
  },

  getNextNumber: async (customerId: string): Promise<{ number: string }> => {
    const response = await fetch(`${API_BASE}/contracts/next?customer_id=${customerId}`);
    return handleResponse<{ number: string }>(response);
  },

  getNextDocNumber: async (contractId: string, type: "invoice" | "act"): Promise<{ number: string }> => {
    const response = await fetch(`${API_BASE}/contracts/${contractId}/next-number?type=${type}`);
    return handleResponse<{ number: string }>(response);
  },

  delete: async (id: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/contracts/${id}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },
};

// API для работы со счетами
export const invoicesAPI = {
  // Получить счета контрагента
  getByCustomer: async (
    customerId: string,
    contractId = "",
    archived: "all" | "true" | "false" = "all",
    page = 1, 
    perPage = 100
  ): Promise<PaginatedResponse<Invoice>> => {
    let url = `${API_BASE}/invoices?customer_id=${customerId}&page=${page}&per_page=${perPage}`;
    if (contractId) {
      url += `&contract_id=${contractId}`;
    }
    if (archived !== "all") {
      url += `&archived=${archived}`;
    }
    const response = await fetch(url);
    return handleResponse<PaginatedResponse<Invoice>>(response);
  },

  // Получить счет по ID
  getById: async (id: string): Promise<SingleResponse<Invoice>> => {
    const response = await fetch(`${API_BASE}/invoices/${id}`);
    return handleResponse<SingleResponse<Invoice>>(response);
  },

  // Получить счет с услугами
  getWithServices: async (id: string): Promise<SingleResponse<InvoiceWithServices>> => {
    const response = await fetch(`${API_BASE}/invoices/${id}/services`);
    return handleResponse<SingleResponse<InvoiceWithServices>>(response);
  },

  // Создать счет
  create: async (data: {
    contract_id?: string;
    customer_id?: string;
    number: string;
    date: string;
    contract_number?: string;
    service_ids?: string[];
    services?: { name: string; price: number }[];
  }): Promise<SingleResponse<Invoice>> => {
    const response = await fetch(`${API_BASE}/invoices`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Invoice>>(response);
  },

  // Дублировать счет
  duplicate: async (data: {
    invoice_id: string;
    number: string;
    date: string;
  }): Promise<SingleResponse<Invoice>> => {
    const response = await fetch(`${API_BASE}/invoices/duplicate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Invoice>>(response);
  },

  delete: async (id: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/invoices/${id}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },

  update: async (
    id: string,
    data: { number?: string; date?: string; status?: string; archived?: boolean }
  ): Promise<SingleResponse<Invoice>> => {
    const response = await fetch(`${API_BASE}/invoices/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Invoice>>(response);
  },

  addLine: async (id: string, line: {
    service_id?: string;
    title?: string;
    unit?: string;
    vat?: number;
    price?: number;
    qty?: number;
  }): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/invoices/${id}/lines`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ line }),
    });
    return handleResponse<{ status: string }>(response);
  },

  updateLine: async (id: string, lineId: string, line: {
    service_id?: string;
    title?: string;
    unit?: string;
    vat?: number;
    price?: number;
    qty?: number;
  }): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/invoices/${id}/lines/${lineId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ line }),
    });
    return handleResponse<{ status: string }>(response);
  },

  deleteLine: async (id: string, lineId: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/invoices/${id}/lines/${lineId}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },

  createActFromInvoice: async (id: string, data: { number: string; date: string; status?: string }): Promise<SingleResponse<Act>> => {
    const response = await fetch(`${API_BASE}/invoices/${id}/act`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Act>>(response);
  },
};

// API для работы с актами
export const actsAPI = {
  getByCustomer: async (
    customerId: string,
    contractId = "",
    archived: "all" | "true" | "false" = "all",
    page = 1,
    perPage = 100
  ): Promise<PaginatedResponse<Act>> => {
    let url = `${API_BASE}/acts?customer_id=${customerId}&page=${page}&per_page=${perPage}`;
    if (contractId) {
      url += `&contract_id=${contractId}`;
    }
    if (archived !== "all") {
      url += `&archived=${archived}`;
    }
    const response = await fetch(url);
    return handleResponse<PaginatedResponse<Act>>(response);
  },

  getById: async (id: string): Promise<SingleResponse<Act>> => {
    const response = await fetch(`${API_BASE}/acts/${id}`);
    return handleResponse<SingleResponse<Act>>(response);
  },

  getWithServices: async (id: string): Promise<SingleResponse<ActWithServices>> => {
    const response = await fetch(`${API_BASE}/acts/${id}/services`);
    return handleResponse<SingleResponse<ActWithServices>>(response);
  },

  create: async (data: {
    contract_id?: string;
    customer_id?: string;
    number: string;
    date: string;
    status?: string;
    contract_number?: string;
    service_ids?: string[];
    services?: { name: string; price: number }[];
    invoice_ids?: string[];
  }): Promise<SingleResponse<Act>> => {
    const response = await fetch(`${API_BASE}/acts`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Act>>(response);
  },

  linkInvoices: async (id: string, invoiceIds: string[]): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/acts/${id}/invoices`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ invoice_ids: invoiceIds }),
    });
    return handleResponse<{ status: string }>(response);
  },

  delete: async (id: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/acts/${id}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },

  update: async (
    id: string,
    data: { number?: string; date?: string; status?: string; archived?: boolean }
  ): Promise<SingleResponse<Act>> => {
    const response = await fetch(`${API_BASE}/acts/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Act>>(response);
  },

  addLine: async (id: string, line: {
    service_id?: string;
    title?: string;
    unit?: string;
    vat?: number;
    price?: number;
    qty?: number;
  }): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/acts/${id}/lines`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ line }),
    });
    return handleResponse<{ status: string }>(response);
  },

  updateLine: async (id: string, lineId: string, line: {
    service_id?: string;
    title?: string;
    unit?: string;
    vat?: number;
    price?: number;
    qty?: number;
  }): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/acts/${id}/lines/${lineId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ line }),
    });
    return handleResponse<{ status: string }>(response);
  },

  deleteLine: async (id: string, lineId: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/acts/${id}/lines/${lineId}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },
};

// API для работы с услугами
export const servicesAPI = {
  getAll: async (page = 1, perPage = 100): Promise<PaginatedResponse<Service>> => {
    const response = await fetch(`${API_BASE}/services?page=${page}&per_page=${perPage}`);
    return handleResponse<PaginatedResponse<Service>>(response);
  },

  // Создать услугу
  create: async (data: { name: string; price: number }): Promise<SingleResponse<Service>> => {
    const response = await fetch(`${API_BASE}/services`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<Service>>(response);
  },

  delete: async (id: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/services/${id}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },
};

// Каталог типовых услуг (стандартный прайс), сгруппированный по разделам
export const serviceCatalogAPI = {
  get: async (): Promise<{ data: ServiceCatalogSection[] }> => {
    const response = await fetch(`${API_BASE}/services/catalog`);
    return handleResponse<{ data: ServiceCatalogSection[] }>(response);
  },
};

// API для работы с приложениями к договору (смета)
export const contractAppendicesAPI = {
  getByContract: async (contractId: string): Promise<PaginatedResponse<ContractAppendix>> => {
    const response = await fetch(`${API_BASE}/contracts/${contractId}/appendices`);
    return handleResponse<PaginatedResponse<ContractAppendix>>(response);
  },

  getNextNumber: async (contractId: string): Promise<{ number: string }> => {
    const response = await fetch(`${API_BASE}/contracts/${contractId}/appendices/next-number`);
    return handleResponse<{ number: string }>(response);
  },

  create: async (
    contractId: string,
    data: { number: string; date: string; status?: string; lines: ContractAppendixLineInput[] }
  ): Promise<SingleResponse<ContractAppendixWithLines>> => {
    const response = await fetch(`${API_BASE}/contracts/${contractId}/appendices`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<ContractAppendixWithLines>>(response);
  },

  getById: async (id: string): Promise<SingleResponse<ContractAppendixWithLines>> => {
    const response = await fetch(`${API_BASE}/contract-appendices/${id}`);
    return handleResponse<SingleResponse<ContractAppendixWithLines>>(response);
  },

  update: async (
    id: string,
    data: { number?: string; date?: string; status?: string; archived?: boolean }
  ): Promise<SingleResponse<ContractAppendix>> => {
    const response = await fetch(`${API_BASE}/contract-appendices/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return handleResponse<SingleResponse<ContractAppendix>>(response);
  },

  delete: async (id: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/contract-appendices/${id}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },

  addLine: async (id: string, line: ContractAppendixLineInput): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/contract-appendices/${id}/lines`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ line }),
    });
    return handleResponse<{ status: string }>(response);
  },

  deleteLine: async (id: string, lineId: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/contract-appendices/${id}/lines/${lineId}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
  },
};
