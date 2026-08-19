// Документы: контрагенты, договоры, счета, акты, услуги, приложения к договору.
import { API_BASE, handleResponse, PaginatedResponse, SingleResponse, NullableSingleResponse } from './client';
import { RedmineDocumentStatus, RedmineProjectLink } from './redmine';

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

export interface InvoiceWithServices extends Invoice {
  services: Service[];
}

export interface ActWithServices extends Act {
  services: Service[];
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

  // Удалить контрагента (сервер отклонит, если есть связанные договоры/счета)
  delete: async (id: string): Promise<{ status: string }> => {
    const response = await fetch(`${API_BASE}/customers/${id}`, {
      method: 'DELETE',
    });
    return handleResponse<{ status: string }>(response);
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

  // Следующий номер акта из реального реестра (Google-таблица), не из
  // внутренней последовательности work_app
  getNextNumberFromSheet: async (): Promise<{ data: { number: string; row: number } }> => {
    const response = await fetch(`${API_BASE}/acts/next-number-from-sheet`);
    return handleResponse<{ data: { number: string; row: number } }>(response);
  },

  // Дописывает строку в реестр для уже созданного акта
  registerInSheet: async (id: string): Promise<{ data: { row: number; number: string; updated_cells: number } }> => {
    const response = await fetch(`${API_BASE}/acts/${id}/register-in-sheet`, {
      method: 'POST',
    });
    return handleResponse<{ data: { row: number; number: string; updated_cells: number } }>(response);
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

// Поля договора, извлечённые из загруженного скана (не более чем найдено —
// пустые поля означают "не распознано", а не догадку).
export interface ParsedContractFields {
  number?: string;
  date?: string; // YYYY-MM-DD
  candidate_inn?: string[];
}

export interface ParsedContractDocument {
  fields: ParsedContractFields;
  pages: number;
  doc_type: string;
}

export const documentsAPI = {
  // Загружает скан договора и возвращает распознанные поля для предзаполнения формы.
  parseContract: async (file: File): Promise<SingleResponse<ParsedContractDocument>> => {
    const formData = new FormData();
    formData.append('file', file);
    const response = await fetch(`${API_BASE}/documents/parse-contract`, {
      method: 'POST',
      body: formData,
    });
    return handleResponse<SingleResponse<ParsedContractDocument>>(response);
  },
};
