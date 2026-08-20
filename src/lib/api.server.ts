// Server-side API client (no cache)
import { ApiError } from "./api/client"
export { ApiError } from "./api/client"

const API_BASE = process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080/api"

type FetchOptions = RequestInit & { cache?: RequestCache; next?: { revalidate?: number } }

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Network error", code: response.status }))
    throw new ApiError(error.error, response.status)
  }
  return response.json()
}

async function apiFetch<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    cache: "no-store",
    next: { revalidate: 0 },
  })
  return handleResponse<T>(response)
}

export interface Customer {
  id: string
  name: string
  fullname: string
  address: string
  inn: string
  kpp: string
  created_at: string
  updated_at: string
}

export interface Contract {
  id: string
  customer_id: string
  number: string
  currency: string
  status: string
  topic: string
  start_date: string
  end_date: string
  created_at: string
  updated_at: string
}

export interface ContractAppendixLine {
  id: string
  appendix_id: string
  service_id?: string
  section: string
  position: number
  title: string
  unit: string
  price: number
  qty: number
  amount: number
}

export interface ContractAppendix {
  id: string
  contract_id: string
  number: string
  date: string
  status: string
  total_amount: number
  archived: boolean
  created_at: string
  updated_at: string
}

export interface ContractAppendixWithLines extends ContractAppendix {
  lines: ContractAppendixLine[]
}

export interface Invoice {
  id: string
  contract_id: string
  customer_id: string
  number: string
  date: string
  status: string
  total_amount: number
  archived: boolean
  contract_number: string
  created_at: string
  updated_at: string
}

export interface Act {
  id: string
  contract_id: string
  customer_id: string
  number: string
  date: string
  status: string
  total_amount: number
  archived: boolean
  contract_number: string
  created_at: string
  updated_at: string
}

export interface Service {
  id: string
  name: string
  unit?: string
  price: number
  qty?: number
  amount?: number
  created_at: string
  updated_at: string
}

export interface InvoiceWithServices extends Invoice {
  services: Service[]
}

export interface ActWithServices extends Act {
  services: Service[]
}

export interface OrganizationSigner {
  position: string
  last_name: string
  first_name: string
  middle_name: string
}

export interface Organization {
  id: string
  type: string
  full_name: string
  short_name: string
  inn: string
  kpp?: string
  ogrn?: string
  legal_address?: string
  postal_address?: string
  phone?: string
  bank_account?: string
  bank_name?: string
  bank_bik?: string
  bank_corr_account?: string
  signer: OrganizationSigner
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  per_page: number
}

export interface SingleResponse<T> {
  data: T
}

export const customersAPI = {
  getById: async (id: string): Promise<SingleResponse<Customer>> => {
    return apiFetch<SingleResponse<Customer>>(`/customers/${id}`)
  },
}

export const invoicesAPI = {
  getById: async (id: string): Promise<SingleResponse<Invoice>> => {
    return apiFetch<SingleResponse<Invoice>>(`/invoices/${id}`)
  },
  getWithServices: async (id: string): Promise<SingleResponse<InvoiceWithServices>> => {
    return apiFetch<SingleResponse<InvoiceWithServices>>(`/invoices/${id}/services`)
  },
}

export const actsAPI = {
  getById: async (id: string): Promise<SingleResponse<Act>> => {
    return apiFetch<SingleResponse<Act>>(`/acts/${id}`)
  },
  getWithServices: async (id: string): Promise<SingleResponse<ActWithServices>> => {
    return apiFetch<SingleResponse<ActWithServices>>(`/acts/${id}/services`)
  },
}

export const contractAppendicesAPI = {
  getById: async (id: string): Promise<SingleResponse<ContractAppendixWithLines>> => {
    return apiFetch<SingleResponse<ContractAppendixWithLines>>(`/contract-appendices/${id}`)
  },
}

export const organizationAPI = {
  get: async (): Promise<SingleResponse<Organization>> => {
    return apiFetch<SingleResponse<Organization>>(`/organization`)
  },
}

export const contractsAPI = {
  getById: async (id: string): Promise<SingleResponse<Contract>> => {
    return apiFetch<SingleResponse<Contract>>(`/contracts/${id}`)
  },
}
