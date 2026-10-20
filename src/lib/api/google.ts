// API Google-авторизации sheets-sync (через него идут номера счетов/актов
// из Google-таблиц). Токен хранится на стороне hermes, work_app его не видит.
import { API_BASE, handleResponse } from './client';

export interface GoogleAuthStatus {
  authenticated: boolean;
  // AUTHENTICATED | NOT_AUTHENTICATED | TOKEN_REVOKED | REFRESH_FAILED | ...
  state: string;
  message: string;
  // ISO-время последнего подключения через интерфейс (null — неизвестно)
  authorized_at: string | null;
  has_client_secret: boolean;
}

export const googleAuthAPI = {
  getStatus: async (): Promise<{ data: GoogleAuthStatus }> => {
    const response = await fetch(`${API_BASE}/google/status`);
    return handleResponse<{ data: GoogleAuthStatus }>(response);
  },

  // Начинает новую OAuth-сессию и возвращает ссылку на экран согласия Google
  getAuthUrl: async (): Promise<{ data: { url: string } }> => {
    const response = await fetch(`${API_BASE}/google/auth-url`, { method: 'POST' });
    return handleResponse<{ data: { url: string } }>(response);
  },

  // code — адрес http://localhost:1/?code=… из браузера после согласия (или сам code)
  submitCode: async (code: string): Promise<{ data: GoogleAuthStatus }> => {
    const response = await fetch(`${API_BASE}/google/auth-code`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code }),
    });
    return handleResponse<{ data: GoogleAuthStatus }>(response);
  },
};
