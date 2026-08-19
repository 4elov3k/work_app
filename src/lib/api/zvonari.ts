// Звонари: звонки из OnlinePBX + транскрибация/аналитика через Hermes
import { API_BASE, handleResponse, SingleResponse } from './client';

export interface Caller {
  id: string;
  pbx_extension: string;
  name: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CallerReport {
  id: string;
  caller_id: string;
  period: string;
  period_start: string;
  period_end: string;
  summary_text: string;
  metrics_json?: unknown;
  requested_at: string;
}

// CallStepStatus — статусы шагов 1/2/3/5/6 по регламенту IQ-200 v1.2
// ("Частично" запрещён для шагов 1, 5 и 6 — см. baza_znaniy_ocenka_zvonkov_po_skriptu_IQ200_v1.2.md).
export type CallStepStatus =
  | "Выполнен"
  | "Частично"
  | "Не выполнен"
  | "Выполнен вне последовательности"
  | "Не применим"
  | "Не оценивается / недостаточно данных"
  | "Корректная остановка";

export type CallStep4Status = "Использован" | "Не использован" | "Не применим";

// CallOutcome — закрытый список из 13 значений (регламент §11). Значение
// "" покрывает legacy-записи, проанализированные до перехода на этот
// регламент (плоский формат {category, outcome}), и записи, для которых
// анализ ещё не запускался.
export type CallOutcome =
  | ""
  | "Технический / содержательный диалог не состоялся"
  | "Срыв на шаге 1"
  | "Шаг 1 выполнен"
  | "Срыв на шаге 2"
  | "Шаг 2 выполнен"
  | "Шаг 3 выполнен вне последовательности"
  | "Шаг 3 выполнен"
  | "Корректно выявлено отсутствие потребности"
  | "Согласован конкретный повторный контакт"
  | "Встреча согласована, шаг 5 не выполнен"
  | "Скрипт пройден до шага 6"
  | "Корректная ранняя остановка"
  | "Недостаточно данных для оценки";

export interface CallStepResult {
  status: CallStepStatus;
  evidence?: string;
  missing?: string;
  // step3-специфичные поля
  branch?: "А" | "Б" | null;
  identified_need?: string | null;
  // step5-специфичные поля
  situation_review_mentioned?: boolean;
  plan_mentioned?: boolean;
  // step6-специфичные поля
  date?: string | null;
  time?: string | null;
  format?: string | null;
  channel_or_address?: string | null;
  final_confirmation?: boolean;
}

export interface CallAnalytics {
  call_type?: "технический" | "содержательный" | "недостаточно_данных";
  fraud_suspected?: boolean;
  counterpart_role?: string;
  lpr_confirmed?: "да" | "нет" | "неясно";
  lpr_name?: string | null;
  steps?: {
    step1?: CallStepResult;
    step2?: CallStepResult;
    step3?: CallStepResult;
    step4?: { status: CallStep4Status };
    step5?: CallStepResult;
    step6?: CallStepResult;
  };
  max_step_reached?: number | null;
  break_point?: string;
  outcome?: CallOutcome;
  recommendation?: string;
  confidence?: "высокая" | "средняя" | "низкая";
  note?: string;
  // Legacy-поля до перехода на регламент IQ-200 v1.2 — оставлены как есть,
  // без бэкфилла (см. решение "leave as-is" при переходе на новый формат).
  category?: string;
}

export interface Call {
  id: string;
  pbx_uuid: string;
  caller_id: string | null;
  direction: string;
  counterparty_number: string;
  started_at: string;
  duration_sec: number;
  talk_time_sec: number;
  hangup_cause: string;
  transcript_status: string;
  transcript_text?: string;
  analytics_json?: CallAnalytics;
  created_at: string;
  updated_at: string;
}

export interface SyncCallsResult {
  callers_synced: number;
  calls_found: number;
  calls_new: number;
  calls_skipped: number;
  transcribe_errors: number;
  analyze_errors: number;
}

export interface SyncStatus {
  running: boolean;
  paused: boolean;
  started_at?: string;
  finished_at?: string;
  total_to_process?: number;
  processed?: number;
  result?: SyncCallsResult;
  error?: string;
}

export const zvonariAPI = {
  // Получить список звонарей (синхронизируется из OnlinePBX)
  getCallers: async (): Promise<SingleResponse<Caller[]>> => {
    const response = await fetch(`${API_BASE}/zvonari/callers`);
    return handleResponse<SingleResponse<Caller[]>>(response);
  },

  // Запустить синхронизацию звонков из OnlinePBX за период (YYYY-MM-DD) —
  // работает в фоне на бэкенде, эндпоинт сразу возвращает статус запуска.
  // Прогресс/результат — через getSyncStatus.
  sync: async (from: string, to: string): Promise<SingleResponse<{ status: string }>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/sync?${params.toString()}`, { method: 'POST' });
    return handleResponse<SingleResponse<{ status: string }>>(response);
  },

  // Текущий статус фоновой синхронизации (последний запуск/выполняется ли сейчас)
  getSyncStatus: async (): Promise<SingleResponse<SyncStatus>> => {
    const response = await fetch(`${API_BASE}/zvonari/sync/status`);
    return handleResponse<SingleResponse<SyncStatus>>(response);
  },

  // Приостановить текущую фоновую задачу перед следующим звонком/батчем —
  // прогресс не теряется, можно продолжить с того же места через resumeSync.
  pauseSync: async (): Promise<SingleResponse<SyncStatus>> => {
    const response = await fetch(`${API_BASE}/zvonari/sync/pause`, { method: 'POST' });
    return handleResponse<SingleResponse<SyncStatus>>(response);
  },

  // Снять паузу — задача продолжает с того же места.
  resumeSync: async (): Promise<SingleResponse<SyncStatus>> => {
    const response = await fetch(`${API_BASE}/zvonari/sync/resume`, { method: 'POST' });
    return handleResponse<SingleResponse<SyncStatus>>(response);
  },

  // Число синхронизированных звонков за период по каждому звонарю (для счётчика на карточках)
  getCallCounts: async (from: string, to: string): Promise<SingleResponse<Record<string, number>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/count?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, number>>>(response);
  },

  // Разбивка звонков по transcript_status на каждого звонаря за период
  // (done/failed/no_recording/pending/transcribing) — полная статистика вместо голого счётчика.
  getStatusCounts: async (from: string, to: string): Promise<SingleResponse<Record<string, Record<string, number>>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/status-counts?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, Record<string, number>>>>(response);
  },

  // Массово повторить транскрибацию для всех звонков за период со статусом
  // failed/no_recording/pending/transcribing. Тоже фоновая задача — статус
  // через getSyncStatus.
  retryFailed: async (from: string, to: string): Promise<SingleResponse<{ status: string }>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/retry-failed?${params.toString()}`, { method: 'POST' });
    return handleResponse<SingleResponse<{ status: string }>>(response);
  },

  // Массово ПЕРЕтранскрибировать вообще все звонки периода, включая уже
  // готовые (в отличие от retryFailed) — транскрибация сама предпочитает
  // GPU-бокс, если он настроен и доступен. Для задним числом улучшения
  // качества транскриптов, сделанных раньше на CPU. Тоже фоновая задача.
  retranscribeAllGpu: async (from: string, to: string): Promise<SingleResponse<{ status: string }>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/retranscribe-gpu?${params.toString()}`, { method: 'POST' });
    return handleResponse<SingleResponse<{ status: string }>>(response);
  },

  // Запустить LLM-оценку по скрипту IQ-200 (регламент v1.2) для звонков за
  // период, у которых уже готов транскрипт, но ещё нет анализа —
  // отдельный шаг от синхронизации/транскрибации. Тоже фоновая задача.
  analyzeCalls: async (from: string, to: string): Promise<SingleResponse<{ status: string }>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/analyze?${params.toString()}`, { method: 'POST' });
    return handleResponse<SingleResponse<{ status: string }>>(response);
  },

  // Разбивка по итогам оценки скрипта (закрытый список из 13 значений,
  // регламент IQ-200 v1.2) на каждого звонаря за период одним запросом —
  // для таблицы звонарей без N+1.
  getOutcomeCounts: async (from: string, to: string): Promise<SingleResponse<Record<string, Record<string, number>>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/outcomes?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, Record<string, number>>>>(response);
  },

  // Число звонков с fraud_suspected=true (не сброшенный вовремя
  // автоответчик) на каждого звонаря за период — для выявления
  // АФК-прослушивания автоответчика без отдельного запроса на каждого звонаря.
  getFraudCounts: async (from: string, to: string): Promise<SingleResponse<Record<string, number>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/fraud-counts?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, number>>>(response);
  },

  // Вручную (пере)запустить транскрибацию+аналитику для одного звонка —
  // например, если он застрял в статусе transcribing/failed.
  retranscribeCall: async (callId: string): Promise<SingleResponse<Call>> => {
    const response = await fetch(`${API_BASE}/zvonari/calls/${callId}/transcribe`, { method: 'POST' });
    return handleResponse<SingleResponse<Call>>(response);
  },

  // Детализация звонков звонаря за период (время, направление, транскрипт, категория)
  getCalls: async (callerId: string, from: string, to: string): Promise<SingleResponse<Call[]>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/callers/${callerId}/calls?${params.toString()}`);
    return handleResponse<SingleResponse<Call[]>>(response);
  },

  // Распределение звонков звонаря по категориям (аналитика Hermes) за период
  getDistribution: async (callerId: string, from: string, to: string): Promise<SingleResponse<Record<string, number>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/callers/${callerId}/distribution?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, number>>>(response);
  },

  // Запросить у Hermes сводный отчёт по звонарю за период
  requestReport: async (callerId: string, period: string, from: string, to: string): Promise<SingleResponse<CallerReport>> => {
    const response = await fetch(`${API_BASE}/zvonari/callers/${callerId}/report`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ period, from, to }),
    });
    return handleResponse<SingleResponse<CallerReport>>(response);
  },

  // Прошлые отчёты по звонарю (новые сверху) — то, что уже сохранено в БД
  getReportHistory: async (callerId: string, limit = 20): Promise<SingleResponse<CallerReport[]>> => {
    const params = new URLSearchParams({ limit: String(limit) });
    const response = await fetch(`${API_BASE}/zvonari/callers/${callerId}/reports?${params.toString()}`);
    return handleResponse<SingleResponse<CallerReport[]>>(response);
  },

  // Ссылка на CSV-выгрузку звонков звонаря за период (дата, время,
  // транскрипт, категория, общая оценка за период) — используется напрямую
  // как href для скачивания, не через fetch.
  exportCallsCsvUrl: (callerId: string, from: string, to: string): string => {
    const params = new URLSearchParams({ from, to });
    return `${API_BASE}/zvonari/callers/${callerId}/export.csv?${params.toString()}`;
  },
};
