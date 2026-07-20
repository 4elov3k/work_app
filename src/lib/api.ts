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
  created_at: string;
  updated_at: string;
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

export interface ErrorResponse {
  error: string;
  code: number;
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
    throw new Error(`${message} (HTTP ${response.status})`);
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
    try {
      const response = await fetch(`${API_BASE}/customers?page=${page}&per_page=${perPage}`);
      return handleResponse<PaginatedResponse<Customer>>(response);
    } catch (err) {
      console.warn("customersAPI.getAll failed, returning empty list:", err);
      return { data: [], total: 0, page, per_page: perPage };
    }
  },

  // Поиск контрагентов
  search: async (query: string, page = 1, perPage = 100): Promise<PaginatedResponse<Customer>> => {
    try {
      const response = await fetch(
        `${API_BASE}/customers?search=${encodeURIComponent(query)}&page=${page}&per_page=${perPage}`
      );
      return handleResponse<PaginatedResponse<Customer>>(response);
    } catch (err) {
      console.warn("customersAPI.search failed, returning empty list:", err);
      return { data: [], total: 0, page, per_page: perPage };
    }
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
