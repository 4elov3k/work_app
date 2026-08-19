// Shared fetch plumbing used by every feature's API client
// (documents, redmine, zvonari) — API_BASE resolution, error handling,
// and the generic response envelope types.

// API Base URL — falls back to the page's own hostname so the same build
// works over localhost, LAN, and VPN (Tailscale) without rebuilding.
function resolveApiBase(): string {
  if (process.env.NEXT_PUBLIC_API_URL) return process.env.NEXT_PUBLIC_API_URL;
  if (typeof window !== 'undefined') {
    return `${window.location.protocol}//${window.location.hostname}:8080/api`;
  }
  return 'http://127.0.0.1:8080/api';
}

export const API_BASE = resolveApiBase();

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
export async function handleResponse<T>(response: Response): Promise<T> {
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
