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
  metrics_json?: CallerReportMetrics | null;
  requested_at: string;
}

// Ключи 4 качеств — те же, что в рубрике оценки звонка (hermes
// call_analytics_server.py, CRITERIA_KEYS).
export type CriterionKey = "listening" | "goal_understanding" | "questions" | "objection_handling";

export interface ReportCallRef {
  number: number;
  call_id: string;
  started_at: string;
}

// metrics_json отчёта по звонарю: by_criterion считает hermes по оценкам
// звонков; report — структурированный разбор (с 2026-10-13, у старых
// отчётов его нет — только summary_text и, возможно, by_criterion).
export interface CallerReportMetrics {
  total_calls?: number;
  scored_calls?: number;
  overall_score?: string;
  by_criterion?: Partial<Record<CriterionKey, { met: number; applicable: number }>>;
  report?: {
    summary?: string;
    criteria?: Partial<Record<CriterionKey, {
      label?: string;
      verdict?: string | null;
      comment?: string | null;
      example_call?: ReportCallRef | null;
    }>>;
    strengths?: string[];
    growth_areas?: string[];
    manager_actions?: { action?: string; why?: string; how?: string; calls?: ReportCallRef[] }[];
    calls_to_review?: (ReportCallRef & { why?: string })[];
    focus_next?: string;
  };
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

// CallCriterion — один из 4 критериев оценки (см. CallCriteria ниже).
// met=null значит "в этом звонке не было подходящей ситуации для оценки
// этого критерия" — не засчитывается ни как выполненный, ни как провальный.
export interface CallCriterion {
  met: boolean | null;
  evidence: string;
  comment: string;
}

// CallCriteria — 4-критериальная оценка звонка (2026-09-29, заменила
// 6-шаговый regламент IQ-200 — см. CallStepResult/CallOutcome ниже,
// оставлены для чтения старых, уже проанализированных по regламенту
// звонков, но новые звонки больше не получают эту форму).
export interface CallCriteria {
  listening: CallCriterion;
  goal_understanding: CallCriterion;
  questions: CallCriterion;
  objection_handling: CallCriterion;
}

export interface CallAnalytics {
  // Текущая форма (2026-09-29 онвард) — 4 критерия вместо шагов скрипта.
  criteria?: CallCriteria;
  score?: string; // "X/N" — X критериев met=true из N применимых
  summary?: string;
  recommendation?: string;

  // Всё ниже — старая форма (regламент IQ-200 v1.2, до 2026-09-29). Звонки,
  // проанализированные до перехода, хранят её как есть, без бэкфилла в
  // новую форму — читайте условно, по наличию criteria/score выше.
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
  confidence?: "высокая" | "средняя" | "низкая";
  note?: string;
  // Legacy-поля до перехода на регламент IQ-200 v1.2 — оставлены как есть,
  // без бэкфилла (см. решение "leave as-is" при переходе на новый формат).
  category?: string;
}

// CallTranscriptSegment — один "ход" реплики с таймкодом (transcribe.Segment
// на бэкенде). speaker — номер говорящего при диаризации на стороне Hermes,
// null иначе. Отсутствует (Call.transcript_segments === undefined) на
// звонках, расшифрованных до этого поля — там есть только transcript_text.
export interface CallTranscriptSegment {
  start: number;
  end: number;
  speaker: number | null;
  text: string;
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
  transcript_segments?: CallTranscriptSegment[];
  analytics_json?: CallAnalytics;
  // engine — каким сервисом фактически транскрибирован звонок ("cpu"/"gpu"),
  // пусто пока не транскрибирован.
  engine?: string;
  transcribed_at?: string;
  // error_kind — причина последней ошибки транскрибации
  // (no_recording/download_failed/transcribe_failed), пусто если готов или
  // ещё не обрабатывался.
  error_kind?: string;
  last_error?: string;
  created_at: string;
  updated_at: string;

  // fraud_precheck_* — сигнал, определённый на этапе транскрибации по
  // аудио (см. backend/internal/transcribe/client.go), отдельно от
  // analytics_json.fraud_suspected (та метка исторически приходила из
  // отдельного hermes-анализа по тексту). fraud_precheck_suspected===undefined
  // значит "предфильтр не выполнялся вообще" — не то же самое, что false.
  fraud_precheck_suspected?: boolean;
  fraud_precheck_check?: string;
  fraud_precheck_quote?: string;
  fraud_precheck_position?: "начало" | "конец";
  // fraud_review_status — "pending" (ждёт решения человека) / "confirmed" /
  // "rejected". Единственное, что реально влияет на исключение из оплаты
  // (во внешнем расчёте) — это "confirmed", не сам fraud_precheck_suspected.
  fraud_review_status?: "pending" | "confirmed" | "rejected";
  fraud_reviewed_at?: string;
}

export interface PingResult {
  configured: boolean;
  ok: boolean;
  error?: string;
}

export interface HealthStatus {
  transcribe_cpu: PingResult;
  transcribe_gpu: PingResult;
  analytics: PingResult;
  checked_at: string;
}

export interface RetranscribePreview {
  total: number;
  already_gpu: number;
  only_cpu_total: number;
  estimated_minutes: number;
}

export interface SyncCallsResult {
  callers_synced: number;
  calls_found: number;
  calls_new: number;
  calls_skipped: number;
  transcribe_errors: number;
  analyze_errors: number;
}

export type StageState = "idle" | "running" | "done" | "failed";

// StageStatus — одна из трёх фаз конвейера (sync/transcribe/analyze),
// независимая от того, какая именно фоновая задача сейчас идёт: например
// "transcribe" отражает последнюю транскрибацию независимо от того, была
// ли она частью полного синка или отдельного повтора (задача 1,
// zvonari-improvements.md).
export interface StageStatus {
  state: StageState;
  done?: number;
  total?: number;
  started_at?: string;
  finished_at?: string;
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
  stages?: Record<string, StageStatus>;
}

export const zvonariAPI = {
  // Здоровье внешних сервисов (транскрибация CPU/GPU, аналитика Hermes) —
  // кешируется на бэкенде на 30с, для точек-индикаторов в шапке.
  getHealth: async (): Promise<SingleResponse<HealthStatus>> => {
    const response = await fetch(`${API_BASE}/zvonari/health`);
    return handleResponse<SingleResponse<HealthStatus>>(response);
  },

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

  // В отличие от паузы — реально прерывает задачу (обрывает то, что сейчас
  // выполняется), а не просто останавливает перед следующим шагом.
  // Уже сохранённый прогресс не теряется.
  stopSync: async (): Promise<SingleResponse<SyncStatus>> => {
    const response = await fetch(`${API_BASE}/zvonari/sync/stop`, { method: 'POST' });
    return handleResponse<SingleResponse<SyncStatus>>(response);
  },

  // Число синхронизированных звонков за период по каждому звонарю (для счётчика на карточках)
  getCallCounts: async (from: string, to: string): Promise<SingleResponse<Record<string, number>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/count?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, number>>>(response);
  },

  // Совокупный балл по 4 критериям на каждого звонаря за период (замена
  // старых "Скрипт до конца"/"Срыв на шаге 1", см. CallCriteria выше) —
  // met_true/applicable ПО ВСЕМ звонкам и критериям вместе, не среднее по
  // отдельным звонкам (см. database.CriteriaScore на бэкенде).
  getCriteriaScores: async (from: string, to: string): Promise<SingleResponse<Record<string, { met_true: number; applicable: number }>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/criteria-scores?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, { met_true: number; applicable: number }>>>(response);
  },

  // Очередь звонков-кандидатов на фрод (fraud_review_status='pending') за
  // период, по всем звонарям — для UI подтверждения/отклонения.
  getFraudCandidates: async (from: string, to: string): Promise<SingleResponse<Call[]>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/fraud-candidates?${params.toString()}`);
    return handleResponse<SingleResponse<Call[]>>(response);
  },

  // Зафиксировать решение человека по кандидату — само по себе ничего не
  // исключает из оплаты (это делает внешний расчёт по "confirmed"), просто
  // сохраняет вердикт.
  reviewFraudCandidate: async (callId: string, status: "confirmed" | "rejected"): Promise<SingleResponse<{ status: string }>> => {
    const response = await fetch(`${API_BASE}/zvonari/calls/${callId}/fraud-review`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    });
    return handleResponse<SingleResponse<{ status: string }>>(response);
  },

  // Ссылка на выгрузку всех фрод-кандидатов за период (включая уже
  // проверенные человеком) в .xlsx — для внешнего расчёта оплаты.
  fraudExportXlsxUrl: (from: string, to: string): string => {
    const params = new URLSearchParams({ from, to });
    return `${API_BASE}/zvonari/calls/fraud-export.xlsx?${params.toString()}`;
  },

  // Разбивка звонков по transcript_status на каждого звонаря за период
  // (done/failed/no_recording/pending/transcribing) — полная статистика вместо голого счётчика.
  getStatusCounts: async (from: string, to: string): Promise<SingleResponse<Record<string, Record<string, number>>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/status-counts?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, Record<string, number>>>>(response);
  },

  // Массово повторить транскрибацию для всех звонков за период с
  // восстановимым статусом (failed/pending/transcribing) — includeTerminal
  // дополнительно берёт no_recording, что обычно бессмысленно (запись
  // действительно отсутствует), поэтому отдельный явный выбор, а не
  // умолчание. Тоже фоновая задача — статус через getSyncStatus.
  retryFailed: async (from: string, to: string, includeTerminal: boolean): Promise<SingleResponse<{ status: string }>> => {
    const params = new URLSearchParams({ from, to, include_terminal: String(includeTerminal) });
    const response = await fetch(`${API_BASE}/zvonari/calls/retry-failed?${params.toString()}`, { method: 'POST' });
    return handleResponse<SingleResponse<{ status: string }>>(response);
  },

  // Разбивка неудачных звонков по причине (error_kind) за период, по всем
  // звонарям — сводка под блоком синхронизации.
  getErrorBreakdown: async (from: string, to: string): Promise<SingleResponse<Record<string, number>>> => {
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${API_BASE}/zvonari/calls/error-breakdown?${params.toString()}`);
    return handleResponse<SingleResponse<Record<string, number>>>(response);
  },

  // Оценка перед запуском перетранскрибации: сколько звонков реально
  // попадёт в перегон и сколько из них уже было на GPU — для диалога
  // подтверждения, чтобы случайный клик не гонял тысячи звонков вслепую.
  getRetranscribePreview: async (from: string, to: string, onlyCpu: boolean): Promise<SingleResponse<RetranscribePreview>> => {
    const params = new URLSearchParams({ from, to, only_cpu: String(onlyCpu) });
    const response = await fetch(`${API_BASE}/zvonari/calls/retranscribe-gpu/preview?${params.toString()}`);
    return handleResponse<SingleResponse<RetranscribePreview>>(response);
  },

  // Массово ПЕРЕтранскрибировать звонки периода — по умолчанию только те, что
  // ещё не были на GPU (onlyCpu=true), либо вообще все при onlyCpu=false,
  // включая уже готовые (в отличие от retryFailed) — транскрибация сама
  // предпочитает GPU-бокс, если он настроен и доступен. Тоже фоновая задача.
  retranscribeAllGpu: async (from: string, to: string, onlyCpu: boolean): Promise<SingleResponse<{ status: string }>> => {
    const params = new URLSearchParams({ from, to, only_cpu: String(onlyCpu) });
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

  // Вручную (пере)запустить только аналитику Hermes для одного звонка (без
  // ретранскрибации) — нужен для уже проанализированных звонков, которые
  // обычный AnalyzeCalls по периоду пропускает.
  analyzeCall: async (callId: string): Promise<SingleResponse<Call>> => {
    const response = await fetch(`${API_BASE}/zvonari/calls/${callId}/analyze`, { method: 'POST' });
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

  // Ссылка на прослушивание записи одного звонка — бэкенд резолвит свежую
  // подписанную ссылку OnlinePBX и редиректит на неё, так что этот URL
  // можно открывать напрямую (window.open), без fetch.
  recordingUrl: (callId: string): string => `${API_BASE}/zvonari/calls/${callId}/recording`,
};
