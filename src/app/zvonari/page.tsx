"use client"
import { Fragment, ReactNode, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  RefreshCw,
  FileBarChart,
  Phone,
  Mic,
  Loader2,
  Sparkles,
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  ChevronRight,
  Search,
  Download,
  History,
  ListChecks,
  Target,
  Ban,
  ShieldAlert,
  HelpCircle,
  Pause,
  Play,
  CheckCircle2,
  XCircle,
  CircleDashed,
  CornerDownRight,
  Cpu,
} from "lucide-react";

import { zvonariAPI, Caller, CallerReport, Call, CallAnalytics, ApiError } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";

// Период (from/to) переживает перезагрузку страницы через localStorage —
// без этого при каждом обновлении фильтр молча слетал обратно на дефолтную
// неделю, теряя то, что реально смотрел человек.
const PERIOD_STORAGE_KEY = "zvonari_period";

function loadStoredPeriod(): { from: string; to: string } | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(PERIOD_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (typeof parsed?.from === "string" && typeof parsed?.to === "string") return parsed;
  } catch {
    // corrupted/old value — ignore and fall back to default period
  }
  return null;
}

function todayISO(offsetDays = 0): string {
  const d = new Date();
  d.setDate(d.getDate() + offsetDays);
  return d.toISOString().slice(0, 10);
}

const PERIOD_PRESETS: { label: string; from: () => string; to: () => string }[] = [
  { label: "Сегодня", from: () => todayISO(), to: () => todayISO() },
  { label: "Неделя", from: () => todayISO(-6), to: () => todayISO() },
  { label: "Месяц", from: () => todayISO(-29), to: () => todayISO() },
];

const DIRECTION_LABELS: Record<string, string> = {
  outbound: "исходящий",
  inbound: "входящий",
  local: "внутренний",
};

const STATUS_LABELS: Record<string, string> = {
  done: "готово",
  failed: "ошибка",
  no_recording: "без записи",
  pending: "в очереди",
  transcribing: "обрабатывается",
};

const STATUS_ORDER = ["done", "transcribing", "pending", "failed", "no_recording"];

// Итог "Скрипт пройден до шага 6" — полное прохождение обязательных шагов
// скрипта (регламент IQ-200 v1.2, §11) — и "Срыв на шаге 1" — самый ранний
// возможный срыв — используются как headline-метрики звонаря, аналогично
// прежним "заинтересован"/"отказ", но привязаны к самому скрипту, а не к
// эмоциональной реакции клиента.
const OUTCOME_SCRIPT_COMPLETED = "Скрипт пройден до шага 6";
const OUTCOME_STEP1_BROKEN = "Срыв на шаге 1";

const CALL_TYPE_LABELS: Record<string, string> = {
  "технический": "технический",
  "содержательный": "содержательный",
  "недостаточно_данных": "недостаточно данных",
};

const STEP_LABELS: Record<string, string> = {
  step1: "Шаг 1 — выход на ЛПР",
  step2: "Шаг 2 — знакомство",
  step3: "Шаг 3 — первичная потребность",
  step4: "Шаг 4 — вилка времени",
  step5: "Шаг 5 — предмет встречи",
  step6: "Шаг 6 — фиксация встречи",
};

const STEP_KEYS: (keyof NonNullable<CallAnalytics["steps"]>)[] = [
  "step1",
  "step2",
  "step3",
  "step4",
  "step5",
  "step6",
];

// Визуальное представление статуса шага (регламент §3) — иконка и цвет,
// единые для всех мест, где отображается статус шага.
function stepStatusMeta(status: string | undefined): { icon: ReactNode; className: string } {
  switch (status) {
    case "Выполнен":
      return { icon: <CheckCircle2 className="h-3.5 w-3.5" />, className: "text-success" };
    case "Использован":
      return { icon: <CheckCircle2 className="h-3.5 w-3.5" />, className: "text-success" };
    case "Частично":
      return { icon: <CircleDashed className="h-3.5 w-3.5" />, className: "text-warning" };
    case "Не выполнен":
    case "Не использован":
      return { icon: <XCircle className="h-3.5 w-3.5" />, className: "text-destructive" };
    case "Выполнен вне последовательности":
      return { icon: <CornerDownRight className="h-3.5 w-3.5" />, className: "text-warning" };
    case "Корректная остановка":
      return { icon: <CheckCircle2 className="h-3.5 w-3.5" />, className: "text-muted-foreground" };
    case "Не применим":
      return { icon: <Ban className="h-3.5 w-3.5" />, className: "text-muted-foreground" };
    default:
      return { icon: <HelpCircle className="h-3.5 w-3.5" />, className: "text-muted-foreground" };
  }
}

// Заливка для точки в компактном индикаторе прогресса (StepStepper) —
// упрощённая версия stepStatusMeta без иконок, для сканирования одним взглядом.
function stepDotClassName(stepKey: string, status: string | undefined): string {
  if (stepKey === "step4") {
    return status === "Использован" ? "bg-primary" : "bg-muted border border-border";
  }
  switch (status) {
    case "Выполнен":
      return "bg-success";
    case "Частично":
      return "bg-warning";
    case "Не выполнен":
      return "bg-destructive";
    case "Выполнен вне последовательности":
      return "bg-warning ring-2 ring-warning/30";
    case "Корректная остановка":
      return "bg-muted-foreground/50";
    case "Не применим":
      return "bg-muted border border-border";
    default:
      return "bg-muted border border-dashed border-border";
  }
}

// Компактный индикатор прогресса по 6 шагам скрипта — точка на шаг, цвет по
// статусу (см. stepDotClassName). Даёт "просканировать" таблицу одним
// взглядом без разворачивания каждой строки; title на каждой точке — статус
// конкретного шага для наведения.
function StepStepper({ steps }: { steps?: CallAnalytics["steps"] }) {
  if (!steps) return <span className="text-xs text-muted-foreground">—</span>;
  return (
    <div className="flex items-center gap-1">
      {STEP_KEYS.map((key) => {
        const status = steps[key]?.status;
        return (
          <span
            key={key}
            title={`${STEP_LABELS[key]}: ${status || "нет данных"}`}
            className={`h-2.5 w-2.5 shrink-0 rounded-full ${stepDotClassName(key, status)}`}
          />
        );
      })}
    </div>
  );
}

// Грубая классификация итога звонка на "тон" — чтобы таблицу можно было
// сканировать по цвету, а не перечитывать текст каждой строки.
function outcomeTone(outcome: string | undefined): "positive" | "negative" | "neutral" | "muted" {
  switch (outcome) {
    case OUTCOME_SCRIPT_COMPLETED:
    case "Согласован конкретный повторный контакт":
    case "Корректно выявлено отсутствие потребности":
      return "positive";
    case OUTCOME_STEP1_BROKEN:
    case "Срыв на шаге 2":
    case "Встреча согласована, шаг 5 не выполнен":
      return "negative";
    case "Технический / содержательный диалог не состоялся":
    case "Недостаточно данных для оценки":
    case undefined:
      return "muted";
    default:
      return "neutral";
  }
}

const OUTCOME_TONE_CLASSES: Record<ReturnType<typeof outcomeTone>, string> = {
  positive: "border-success/40 bg-success/10 text-success",
  negative: "border-destructive/40 bg-destructive/10 text-destructive",
  neutral: "border-border text-foreground",
  muted: "border-border text-muted-foreground",
};

function OutcomeBadge({ outcome }: { outcome?: string }) {
  if (!outcome) return <span className="text-xs text-muted-foreground">не проанализировано</span>;
  return (
    <Badge variant="outline" className={`whitespace-normal text-left font-normal ${OUTCOME_TONE_CLASSES[outcomeTone(outcome)]}`}>
      {outcome}
    </Badge>
  );
}

function StepBreakdown({ analytics }: { analytics: CallAnalytics }) {
  const steps = analytics.steps;
  if (!steps) return null;
  return (
    <div className="mt-2 space-y-2 rounded-md border border-border/70 bg-card p-2">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
        {analytics.counterpart_role && <span>Собеседник: {analytics.counterpart_role}</span>}
        {analytics.lpr_confirmed && <span>ЛПР подтверждён: {analytics.lpr_confirmed}</span>}
        {analytics.lpr_name && <span>Имя ЛПР: {analytics.lpr_name}</span>}
        {typeof analytics.max_step_reached === "number" && (
          <span className="inline-flex items-center gap-1">
            <Target className="h-3 w-3" />
            Макс. пройденный шаг: {analytics.max_step_reached}/6
          </span>
        )}
        {analytics.confidence && <span>Уверенность: {analytics.confidence}</span>}
      </div>
      <div className="space-y-1.5">
        {STEP_KEYS.map((key) => {
          const step = steps[key];
          if (!step) return null;
          const meta = stepStatusMeta(step.status);
          return (
            <div key={key} className="flex flex-col gap-0.5 border-t border-border/50 pt-1.5 text-sm first:border-t-0 first:pt-0">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className={`inline-flex items-center gap-1 font-medium ${meta.className}`}>
                  {meta.icon}
                  {STEP_LABELS[key]}
                </span>
                <span className={`text-xs ${meta.className}`}>{step.status}</span>
              </div>
              {"evidence" in step && step.evidence && (
                <p className="text-xs text-muted-foreground">Доказательство: {step.evidence}</p>
              )}
              {"missing" in step && step.missing && step.status !== "Выполнен" && (
                <p className="text-xs text-muted-foreground">Чего не хватило: {step.missing}</p>
              )}
            </div>
          );
        })}
      </div>
      {(analytics.break_point || analytics.recommendation) && (
        <div className="space-y-1 border-t border-border/50 pt-1.5 text-xs">
          {analytics.break_point && (
            <p className="text-muted-foreground">Место срыва: {analytics.break_point}</p>
          )}
          {analytics.recommendation && (
            <p className="text-foreground">Рекомендация: {analytics.recommendation}</p>
          )}
        </div>
      )}
    </div>
  );
}

// Порог, начиная с которого звонарь помечается как "требует внимания" —
// доля ошибок/без записи от всех его звонков за период. Игнорируем совсем
// маленькие выборки (<5 звонков), чтобы один неудачный звонок не красил
// звонаря с 1-2 звонками в красный без статистической значимости.
const PROBLEM_RATIO_THRESHOLD = 0.3;
const PROBLEM_MIN_CALLS = 5;

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

function DistributionBars({ distribution }: { distribution: Record<string, number> }) {
  const entries = Object.entries(distribution);
  const max = Math.max(1, ...entries.map(([, count]) => count));
  if (entries.length === 0) {
    return <p className="text-sm text-muted-foreground">Нет звонков за этот период</p>;
  }
  return (
    <div className="space-y-2">
      {entries.map(([outcome, count]) => (
        <div key={outcome} className="flex items-center gap-3 text-sm">
          <span className="w-40 shrink-0 truncate text-muted-foreground">{outcome}</span>
          <div className="h-3 flex-1 rounded bg-muted overflow-hidden">
            <div className="h-full bg-primary" style={{ width: `${(count / max) * 100}%` }} />
          </div>
          <span className="w-6 shrink-0 text-right font-medium">{count}</span>
        </div>
      ))}
    </div>
  );
}

interface CallFilters {
  status: string;
  callType: string;
  outcome: string;
  direction: string;
  search: string;
  fraudOnly: boolean;
}

const EMPTY_FILTERS: CallFilters = { status: "", callType: "", outcome: "", direction: "", search: "", fraudOnly: false };

// CallOutcome uses "" to mean "not yet analyzed" (see zvonari.ts), which
// collides with the filter's own "no filter selected" sentinel — this
// separate value lets the dropdown offer "unanalyzed" as its own option
// instead of it being unreachable.
const UNANALYZED_OUTCOME = "__unanalyzed__";

function filterCalls(calls: Call[], filters: CallFilters): Call[] {
  return calls.filter((call) => {
    if (filters.status && call.transcript_status !== filters.status) return false;
    if (filters.callType && (call.analytics_json?.call_type || "") !== filters.callType) return false;
    if (filters.outcome === UNANALYZED_OUTCOME) {
      if (call.analytics_json?.outcome) return false;
    } else if (filters.outcome && (call.analytics_json?.outcome || "") !== filters.outcome) {
      return false;
    }
    if (filters.fraudOnly && !call.analytics_json?.fraud_suspected) return false;
    if (filters.direction && call.direction !== filters.direction) return false;
    if (filters.search) {
      const q = filters.search.toLowerCase();
      const inTranscript = (call.transcript_text || "").toLowerCase().includes(q);
      const inNumber = call.counterparty_number.includes(filters.search);
      if (!inTranscript && !inNumber) return false;
    }
    return true;
  });
}

const CALLS_PAGE_SIZE = 50;

function CallDetailList({
  calls,
  retranscribingIds,
  onRetranscribe,
}: {
  calls: Call[];
  retranscribingIds: Set<string>;
  onRetranscribe: (callId: string) => void;
}) {
  const [filters, setFilters] = useState<CallFilters>(EMPTY_FILTERS);
  const [expandedCallId, setExpandedCallId] = useState<string | null>(null);
  const [page, setPage] = useState(0);
  const filtered = useMemo(() => filterCalls(calls, filters), [calls, filters]);
  // Some periods return hundreds of calls (each with a full transcript) —
  // rendering them all into one unvirtualized table is genuinely slow, so
  // paginate client-side rather than pulling in a virtualization library.
  const pageCount = Math.max(1, Math.ceil(filtered.length / CALLS_PAGE_SIZE));
  const currentPage = Math.min(page, pageCount - 1);
  const paginated = useMemo(
    () => filtered.slice(currentPage * CALLS_PAGE_SIZE, (currentPage + 1) * CALLS_PAGE_SIZE),
    [filtered, currentPage]
  );
  // Reset to page 1 when the filters or the underlying call list change —
  // done during render (React's documented pattern for this) rather than
  // an effect, which would cause an extra commit.
  const [resetTrackedOn, setResetTrackedOn] = useState({ filters, calls });
  if (resetTrackedOn.filters !== filters || resetTrackedOn.calls !== calls) {
    setResetTrackedOn({ filters, calls });
    setPage(0);
  }

  const outcomes = useMemo(() => {
    const set = new Set<string>();
    calls.forEach((c) => c.analytics_json?.outcome && set.add(c.analytics_json.outcome));
    return Array.from(set).sort();
  }, [calls]);

  const hasUnanalyzed = useMemo(() => calls.some((c) => !c.analytics_json?.outcome), [calls]);

  const callTypes = useMemo(() => {
    const set = new Set<string>();
    calls.forEach((c) => c.analytics_json?.call_type && set.add(c.analytics_json.call_type));
    return Array.from(set).sort();
  }, [calls]);

  const hasFraudSuspected = useMemo(() => calls.some((c) => c.analytics_json?.fraud_suspected), [calls]);

  if (calls.length === 0) {
    return <p className="text-sm text-muted-foreground">Нет звонков за этот период</p>;
  }

  const selectClass =
    "h-8 rounded-md border border-input bg-card px-2 text-sm transition-colors hover:border-ring/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border/70 bg-muted/40 p-2">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Поиск по тексту или номеру"
            value={filters.search}
            onChange={(e) => setFilters((f) => ({ ...f, search: e.target.value }))}
            className="h-8 w-56 bg-card pl-7 text-sm"
          />
        </div>
        <select
          value={filters.status}
          onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все статусы</option>
          {STATUS_ORDER.map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        <select
          value={filters.callType}
          onChange={(e) => setFilters((f) => ({ ...f, callType: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все типы звонков</option>
          {callTypes.map((c) => (
            <option key={c} value={c}>
              {CALL_TYPE_LABELS[c] || c}
            </option>
          ))}
        </select>
        {hasFraudSuspected && (
          <Button
            variant={filters.fraudOnly ? "destructive" : "outline"}
            size="sm"
            className="h-8 gap-1.5 bg-card transition-transform active:scale-95"
            onClick={() => setFilters((f) => ({ ...f, fraudOnly: !f.fraudOnly }))}
          >
            <ShieldAlert className="h-3.5 w-3.5" />
            Только фрод
          </Button>
        )}
        <select
          value={filters.outcome}
          onChange={(e) => setFilters((f) => ({ ...f, outcome: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все исходы</option>
          {hasUnanalyzed && <option value={UNANALYZED_OUTCOME}>Не проанализировано</option>}
          {outcomes.map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </select>
        <select
          value={filters.direction}
          onChange={(e) => setFilters((f) => ({ ...f, direction: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все направления</option>
          {Object.entries(DIRECTION_LABELS).map(([key, label]) => (
            <option key={key} value={key}>
              {label}
            </option>
          ))}
        </select>
        {(filters.status || filters.callType || filters.outcome || filters.direction || filters.search || filters.fraudOnly) && (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => setFilters(EMPTY_FILTERS)}
          >
            Сбросить
          </Button>
        )}
        <span className="ml-auto text-xs text-muted-foreground">
          {filtered.length} из {calls.length}
        </span>
      </div>

      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">Ничего не найдено по этим фильтрам</p>
      ) : (
        <div className="flex items-center gap-3 px-1 text-xs text-muted-foreground">
          <span className="font-medium text-foreground">Прогресс:</span>
          <span className="inline-flex items-center gap-1">
            <span className="h-2.5 w-2.5 rounded-full bg-success" /> выполнен
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="h-2.5 w-2.5 rounded-full bg-warning" /> частично / вне очереди
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="h-2.5 w-2.5 rounded-full bg-destructive" /> не выполнен
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="h-2.5 w-2.5 rounded-full bg-muted border border-dashed border-border" /> н/д или не применим
          </span>
          <span className="ml-auto">1 → 2 → 3 → 4 → 5 → 6</span>
        </div>
      )}

      {filtered.length > 0 && (
        <div className="overflow-hidden rounded-lg border border-border/70">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-6" />
                <TableHead>Время</TableHead>
                <TableHead>Длит.</TableHead>
                <TableHead>Прогресс по скрипту</TableHead>
                <TableHead>Итог</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {paginated.map((call) => {
                const isRetranscribing = retranscribingIds.has(call.id);
                const analytics = call.analytics_json;
                const isFraud = !!analytics?.fraud_suspected;
                const isExpanded = expandedCallId === call.id;
                const notDone = call.transcript_status !== "done";
                return (
                  <Fragment key={call.id}>
                    <TableRow
                      role="button"
                      tabIndex={0}
                      className={`cursor-pointer transition-colors hover:bg-accent/50 ${isExpanded ? "bg-accent/30" : ""} ${
                        isFraud ? "border-l-4 border-l-destructive" : ""
                      }`}
                      onClick={() => setExpandedCallId(isExpanded ? null : call.id)}
                      onKeyDown={(event) => {
                        if (event.target !== event.currentTarget) return
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault()
                          setExpandedCallId(isExpanded ? null : call.id)
                        }
                      }}
                    >
                      <TableCell>
                        <ChevronRight
                          className={`h-4 w-4 text-muted-foreground transition-transform duration-200 ${
                            isExpanded ? "rotate-90 text-primary" : ""
                          }`}
                        />
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-sm">
                        <div>{new Date(call.started_at).toLocaleString("ru-RU")}</div>
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <Badge variant="outline" className="px-1 py-0 text-[10px] font-normal">
                            {DIRECTION_LABELS[call.direction] || call.direction}
                          </Badge>
                          {notDone && (
                            <span className="text-warning">{STATUS_LABELS[call.transcript_status] || call.transcript_status}</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                        {formatDuration(call.duration_sec)}
                      </TableCell>
                      <TableCell>
                        <StepStepper steps={analytics?.steps} />
                      </TableCell>
                      <TableCell className="max-w-xs">
                        <div className="flex flex-wrap items-center gap-1">
                          <OutcomeBadge outcome={analytics?.outcome} />
                          {isFraud && (
                            <Badge variant="destructive" className="gap-1 px-1.5 py-0 text-[10px]">
                              <ShieldAlert className="h-3 w-3" />
                              фрод
                            </Badge>
                          )}
                          {analytics?.confidence && analytics.confidence !== "высокая" && (
                            <span className="text-[10px] text-muted-foreground">увер.: {analytics.confidence}</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2 transition-transform active:scale-95"
                          disabled={isRetranscribing}
                          title="Транскрибировать заново"
                          aria-label="Транскрибировать заново"
                          onClick={() => onRetranscribe(call.id)}
                        >
                          {isRetranscribing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Mic className="h-3.5 w-3.5" />}
                        </Button>
                      </TableCell>
                    </TableRow>
                    {isExpanded && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell colSpan={6} className="bg-accent/10">
                          <div className="space-y-2 py-1">
                            {analytics?.steps && <StepBreakdown analytics={analytics} />}
                            <p className="whitespace-pre-wrap rounded-md bg-muted/50 p-2 text-sm text-muted-foreground">
                              {call.transcript_text || "Транскрипт недоступен"}
                            </p>
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                );
              })}
            </TableBody>
          </Table>
          {pageCount > 1 && (
            <div className="flex items-center justify-between border-t border-border/70 px-3 py-2 text-xs text-muted-foreground">
              <span>
                Стр. {currentPage + 1} из {pageCount}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2"
                  disabled={currentPage === 0}
                  onClick={() => setPage(currentPage - 1)}
                >
                  Назад
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2"
                  disabled={currentPage >= pageCount - 1}
                  onClick={() => setPage(currentPage + 1)}
                >
                  Вперёд
                </Button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

interface CallerPanelState {
  distribution: Record<string, number> | null;
  distributionLoading: boolean;
  reportLoading: boolean;
  reportError: string;
  report: CallerReport | null;
  callsLoading: boolean;
  callsError: string;
  calls: Call[] | null;
  history: CallerReport[] | null;
  historyLoading: boolean;
}

const EMPTY_PANEL: CallerPanelState = {
  distribution: null,
  distributionLoading: false,
  reportLoading: false,
  reportError: "",
  report: null,
  callsLoading: false,
  callsError: "",
  calls: null,
  history: null,
  historyLoading: false,
};

interface CallerStats {
  caller: Caller;
  total: number;
  done: number;
  donePct: number;
  outcomes: Record<string, number>;
  fraudCount: number;
  problemRatio: number;
  isProblem: boolean;
}

type SortKey = "name" | "total" | "donePct" | "scriptCompleted" | "step1Broken" | "fraud" | "problem";

const KPI_ACCENTS = {
  neutral: "border-l-border",
  primary: "border-l-primary",
  positive: "border-l-success",
  negative: "border-l-destructive",
  warning: "border-l-warning",
} as const;

function KpiCard({
  label,
  value,
  hint,
  icon,
  accent = "neutral",
}: {
  label: string;
  value: string;
  hint?: string;
  icon?: ReactNode;
  accent?: keyof typeof KPI_ACCENTS;
}) {
  return (
    <Card
      className={`border-l-4 ${KPI_ACCENTS[accent]} shadow-sm transition-shadow hover:shadow-md`}
    >
      <CardContent className="pt-6">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <div className="text-2xl font-bold tracking-tight">{value}</div>
            <div className="break-words text-sm text-muted-foreground">{label}</div>
          </div>
          {icon && <span className="shrink-0 text-muted-foreground/60">{icon}</span>}
        </div>
        {hint && <div className="mt-1 text-xs text-muted-foreground">{hint}</div>}
      </CardContent>
    </Card>
  );
}

export default function ZvonariPage() {
  const [callers, setCallers] = useState<Caller[]>([]);
  const [loadingCallers, setLoadingCallers] = useState(true);
  const [listError, setListError] = useState("");
  const [aggregatesError, setAggregatesError] = useState("");

  const [from, setFrom] = useState(todayISO(-6));
  const [to, setTo] = useState(todayISO());

  // Reads the persisted period after mount rather than in the useState
  // initializer above — the server has no localStorage, so computing the
  // initial value from it there would make the client's first render
  // disagree with the server-rendered HTML (React hydration mismatch).
  // Loading it post-mount instead means one harmless extra render on the
  // client right after hydration, not a mismatch during hydration itself.
  useEffect(() => {
    const stored = loadStoredPeriod();
    if (stored) {
      setFrom(stored.from);
      setTo(stored.to);
    }
  }, []);

  useEffect(() => {
    window.localStorage.setItem(PERIOD_STORAGE_KEY, JSON.stringify({ from, to }));
  }, [from, to]);

  const [syncing, setSyncing] = useState(false);
  const [paused, setPaused] = useState(false);
  const [pausing, setPausing] = useState(false);
  const [syncError, setSyncError] = useState("");
  const [syncMessage, setSyncMessage] = useState("");
  const [syncProgress, setSyncProgress] = useState<{ total: number; processed: number } | null>(null);

  const [callCounts, setCallCounts] = useState<Record<string, number>>({});
  const [statusCounts, setStatusCounts] = useState<Record<string, Record<string, number>>>({});
  const [outcomeCounts, setOutcomeCounts] = useState<Record<string, Record<string, number>>>({});
  const [fraudCounts, setFraudCounts] = useState<Record<string, number>>({});

  const [panels, setPanels] = useState<Record<string, CallerPanelState>>({});
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [retranscribingIds, setRetranscribingIds] = useState<Set<string>>(new Set());

  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  const period = useMemo(() => ({ from, to }), [from, to]);

  // Tracks the currently-running poll interval so pollSyncStatus can be
  // called safely from multiple places (mount check, 409-race retry,
  // job-start) without ever having two intervals ticking at once, and so
  // it can be torn down on unmount instead of leaking.
  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => {
    return () => {
      if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
    };
  }, []);

  // pollSyncStatus's setInterval closure is created once per background job
  // and would otherwise see stale from/to/expandedId/panels — refs give it a
  // way to read the live values on every tick without re-subscribing.
  const periodRef = useRef(period);
  useEffect(() => {
    periodRef.current = period;
  }, [period]);
  const expandedIdRef = useRef(expandedId);
  useEffect(() => {
    expandedIdRef.current = expandedId;
  }, [expandedId]);
  const panelsRef = useRef(panels);
  useEffect(() => {
    panelsRef.current = panels;
  }, [panels]);

  const loadCallers = () => {
    setLoadingCallers(true);
    zvonariAPI
      .getCallers()
      .then((response) => {
        setCallers(response.data || []);
        setListError("");
      })
      .catch((err) => {
        console.error("Failed to load callers:", err);
        setListError("Не удалось загрузить список звонарей.");
      })
      .finally(() => setLoadingCallers(false));
  };

  const loadAggregates = (periodFrom: string, periodTo: string) => {
    setAggregatesError("");
    zvonariAPI
      .getCallCounts(periodFrom, periodTo)
      .then((response) => setCallCounts(response.data || {}))
      .catch((err) => {
        console.error("Failed to load call counts:", err);
        setAggregatesError("Не удалось загрузить статистику по звонкам — показанные цифры могут быть неполными.");
      });
    zvonariAPI
      .getStatusCounts(periodFrom, periodTo)
      .then((response) => setStatusCounts(response.data || {}))
      .catch((err) => {
        console.error("Failed to load status counts:", err);
        setAggregatesError("Не удалось загрузить статистику по звонкам — показанные цифры могут быть неполными.");
      });
    zvonariAPI
      .getOutcomeCounts(periodFrom, periodTo)
      .then((response) => setOutcomeCounts(response.data || {}))
      .catch((err) => {
        console.error("Failed to load outcome counts:", err);
        setAggregatesError("Не удалось загрузить статистику по звонкам — показанные цифры могут быть неполными.");
      });
    zvonariAPI
      .getFraudCounts(periodFrom, periodTo)
      .then((response) => setFraudCounts(response.data || {}))
      .catch((err) => {
        console.error("Failed to load fraud counts:", err);
        setAggregatesError("Не удалось загрузить статистику по звонкам — показанные цифры могут быть неполными.");
      });
  };

  // Обновляет цифры без перезагрузки страницы и без спиннеров/сброса того,
  // что уже открыто — вызывается на каждый тик поллинга, пока идёт
  // синхронизация/анализ, чтобы видеть прогресс на живых данных, а не только
  // после завершения задачи.
  const refreshLiveData = () => {
    const { from: liveFrom, to: liveTo } = periodRef.current;
    loadAggregates(liveFrom, liveTo);
    const openId = expandedIdRef.current;
    if (!openId) return;
    zvonariAPI
      .getDistribution(openId, liveFrom, liveTo)
      .then((response) => updatePanel(openId, { distribution: response.data }))
      .catch(() => {});
    if (panelsRef.current[openId]?.calls) {
      zvonariAPI
        .getCalls(openId, liveFrom, liveTo)
        .then((response) => updatePanel(openId, { calls: response.data || [] }))
        .catch(() => {});
    }
  };

  // Синхронизация/анализ/повтор идут в фоне на бэкенде (могут занимать
  // минуты — сотни звонков), поэтому опрашиваем статус вместо того, чтобы
  // держать один долгий запрос — иначе прокси/браузер обрывает соединение
  // раньше, чем бэкенд успевает закончить.
  const pollSyncStatus = () => {
    // Idempotent: if a poll is already running (e.g. the mount check and a
    // 409 retry both call this), replace it instead of ticking twice.
    if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
    pollIntervalRef.current = setInterval(async () => {
      try {
        const response = await zvonariAPI.getSyncStatus();
        const status = response.data;
        if (status.total_to_process) {
          setSyncProgress({ total: status.total_to_process, processed: status.processed ?? 0 });
        }
        setPaused(status.paused ?? false);
        refreshLiveData();
        if (!status.running) {
          if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
          pollIntervalRef.current = null;
          setSyncing(false);
          setPaused(false);
          setSyncProgress(null);
          if (status.error) {
            setSyncError(status.error);
          } else if (status.result) {
            const r = status.result;
            const parts = [`найдено: ${r.calls_found}`];
            if (r.calls_new) parts.push(`новых: ${r.calls_new}`);
            if (r.calls_skipped) parts.push(`пропущено: ${r.calls_skipped}`);
            if (r.transcribe_errors) parts.push(`ошибок транскрибации: ${r.transcribe_errors}`);
            if (r.analyze_errors) parts.push(`ошибок анализа: ${r.analyze_errors}`);
            setSyncMessage(parts.join(", "));
          }
          loadCallers();
        }
      } catch (err) {
        console.error("Sync status poll failed:", err);
        if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
        pollIntervalRef.current = null;
        setSyncing(false);
        setSyncProgress(null);
      }
    }, 3000);
  };

  // При открытии страницы проверяем, не идёт ли уже фоновая задача.
  useEffect(() => {
    loadCallers();
    zvonariAPI
      .getSyncStatus()
      .then((response) => {
        if (response.data.running) {
          setSyncing(true);
          setPaused(response.data.paused ?? false);
          setSyncMessage("Задача уже выполняется...");
          pollSyncStatus();
        }
      })
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    loadAggregates(from, to);
    // distribution/calls are fetched per period and cached in `panels`
    // (see toggleExpand's `!panels[next]?.distribution` guard) — without
    // this, an already-cached panel keeps showing the previous period's
    // data forever, disagreeing with the row's own just-updated aggregate
    // columns. `history` (past reports) isn't period-scoped, so it survives.
    const openId = expandedIdRef.current;
    const hadCalls = Boolean(openId && panelsRef.current[openId]?.calls);
    setPanels((previous) => {
      const openPanel = openId ? previous[openId] : undefined;
      if (!openId) return {};
      return { [openId]: { ...EMPTY_PANEL, history: openPanel?.history ?? null } };
    });
    if (openId) {
      updatePanel(openId, { distributionLoading: true });
      zvonariAPI
        .getDistribution(openId, from, to)
        .then((response) => updatePanel(openId, { distribution: response.data, distributionLoading: false }))
        .catch((err) => {
          console.error("Failed to load distribution:", err);
          updatePanel(openId, { distributionLoading: false });
        });
      // Only re-fetch the call list if it was already open — matches
      // refreshLiveData's same panelsRef.current[openId]?.calls check.
      if (hadCalls) handleLoadCalls(openId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [from, to]);

  const runBackgroundJob = async (
    starter: () => Promise<{ data: { status: string } }>,
    startMessage: string
  ) => {
    setSyncing(true);
    setPaused(false);
    setSyncError("");
    setSyncMessage(startMessage);
    try {
      await starter();
      pollSyncStatus();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setSyncMessage("Уже выполняется другая задача, ожидаем завершения...");
        pollSyncStatus();
        return;
      }
      console.error("Background job failed to start:", err);
      setSyncError(err instanceof Error ? err.message : "Не удалось запустить задачу");
      setSyncing(false);
    }
  };

  const handleSync = () =>
    runBackgroundJob(
      () => zvonariAPI.sync(period.from, period.to),
      "Синхронизация запущена, это может занять несколько минут..."
    );
  const handleRetryFailed = () =>
    runBackgroundJob(
      () => zvonariAPI.retryFailed(period.from, period.to),
      "Повтор запущен, это может занять несколько минут..."
    );
  const handleRetranscribeGpu = () =>
    runBackgroundJob(
      () => zvonariAPI.retranscribeAllGpu(period.from, period.to),
      "Перетранскрибация запущена — все звонки периода будут пересняты (GPU, если доступен)..."
    );
  const handleAnalyze = () =>
    runBackgroundJob(
      () => zvonariAPI.analyzeCalls(period.from, period.to),
      "Анализ запущен — классифицируем готовые транскрипты..."
    );

  const handlePauseResume = async () => {
    setPausing(true);
    try {
      if (paused) {
        const response = await zvonariAPI.resumeSync();
        setPaused(response.data.paused);
        setSyncMessage("Продолжаем...");
      } else {
        const response = await zvonariAPI.pauseSync();
        setPaused(response.data.paused);
        setSyncMessage("Приостановлено — прогресс сохранён, можно продолжить в любой момент.");
      }
    } catch (err) {
      console.error("Pause/resume failed:", err);
    } finally {
      setPausing(false);
    }
  };

  const updatePanel = (callerId: string, patch: Partial<CallerPanelState>) => {
    setPanels((current) => ({ ...current, [callerId]: { ...(current[callerId] || EMPTY_PANEL), ...patch } }));
  };

  const toggleExpand = (callerId: string) => {
    const next = expandedId === callerId ? null : callerId;
    setExpandedId(next);
    if (next && !panels[next]?.distribution && !panels[next]?.distributionLoading) {
      updatePanel(next, { distributionLoading: true });
      zvonariAPI
        .getDistribution(next, period.from, period.to)
        .then((response) => updatePanel(next, { distribution: response.data, distributionLoading: false }))
        .catch((err) => {
          console.error("Failed to load distribution:", err);
          updatePanel(next, { distributionLoading: false });
        });
    }
  };

  const handleRequestReport = async (callerId: string) => {
    updatePanel(callerId, { reportLoading: true, reportError: "" });
    try {
      const response = await zvonariAPI.requestReport(callerId, "custom", period.from, period.to);
      updatePanel(callerId, { reportLoading: false, report: response.data });
      // Свежий отчёт только что попал в БД — если история уже была открыта,
      // подтягиваем её заново, чтобы новый отчёт сразу появился в списке.
      if (panels[callerId]?.history) {
        handleLoadHistory(callerId);
      }
    } catch (err) {
      console.error("Report request failed:", err);
      updatePanel(callerId, {
        reportLoading: false,
        reportError: err instanceof Error ? err.message : "Не удалось получить отчёт",
      });
    }
  };

  const handleLoadHistory = async (callerId: string) => {
    updatePanel(callerId, { historyLoading: true });
    try {
      const response = await zvonariAPI.getReportHistory(callerId);
      updatePanel(callerId, { historyLoading: false, history: response.data || [] });
    } catch (err) {
      console.error("Failed to load report history:", err);
      updatePanel(callerId, { historyLoading: false });
    }
  };

  const handleDownloadCsv = async (callerId: string) => {
    // A plain `window.location.href = url` navigates the whole SPA away
    // (losing period/expanded-row/panel state) if the response isn't
    // actually a CSV attachment (e.g. a JSON error body) — fetch and check
    // first, matching the pattern used by the invoice/act XML export.
    const url = zvonariAPI.exportCallsCsvUrl(callerId, period.from, period.to);
    updatePanel(callerId, { callsError: "" });
    try {
      const response = await fetch(url);
      if (!response.ok) {
        const text = await response.text().catch(() => "");
        throw new Error(text || `Ошибка HTTP ${response.status}`);
      }
      const disposition = response.headers.get("content-disposition");
      const match = disposition?.match(/filename\*=UTF-8''([^;]+)/i) || disposition?.match(/filename="?([^";]+)"?/i);
      const filename = match?.[1] ? decodeURIComponent(match[1]) : "calls.csv";
      const blob = await response.blob();
      const blobUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = blobUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(blobUrl);
    } catch (err) {
      console.error("CSV export failed:", err);
      updatePanel(callerId, {
        callsError: err instanceof Error ? err.message : "Не удалось выгрузить CSV",
      });
    }
  };

  const handleLoadCalls = async (callerId: string) => {
    updatePanel(callerId, { callsLoading: true, callsError: "" });
    try {
      const response = await zvonariAPI.getCalls(callerId, period.from, period.to);
      updatePanel(callerId, { callsLoading: false, calls: response.data || [] });
    } catch (err) {
      console.error("Failed to load calls:", err);
      updatePanel(callerId, {
        callsLoading: false,
        callsError: err instanceof Error ? err.message : "Не удалось загрузить звонки",
      });
    }
  };

  const handleRetranscribeCall = async (callerId: string, callId: string) => {
    setRetranscribingIds((current) => new Set(current).add(callId));
    updatePanel(callerId, { callsError: "" });
    try {
      const response = await zvonariAPI.retranscribeCall(callId);
      const updated = response.data;
      setPanels((current) => {
        const panel = current[callerId];
        if (!panel?.calls) return current;
        return { ...current, [callerId]: { ...panel, calls: panel.calls.map((c) => (c.id === callId ? updated : c)) } };
      });
    } catch (err) {
      console.error("Retranscribe failed:", err);
      updatePanel(callerId, {
        callsError: err instanceof Error ? err.message : "Не удалось перетранскрибировать звонок",
      });
    } finally {
      setRetranscribingIds((current) => {
        const next = new Set(current);
        next.delete(callId);
        return next;
      });
    }
  };

  const callerStats: CallerStats[] = useMemo(() => {
    return callers.map((caller) => {
      const total = callCounts[caller.id] ?? 0;
      const statuses = statusCounts[caller.id] ?? {};
      const outcomes = outcomeCounts[caller.id] ?? {};
      const done = statuses.done ?? 0;
      const problems = (statuses.failed ?? 0) + (statuses.no_recording ?? 0);
      const problemRatio = total > 0 ? problems / total : 0;
      return {
        caller,
        total,
        done,
        donePct: total > 0 ? Math.round((done / total) * 100) : 0,
        outcomes,
        fraudCount: fraudCounts[caller.id] ?? 0,
        problemRatio,
        isProblem: total >= PROBLEM_MIN_CALLS && problemRatio >= PROBLEM_RATIO_THRESHOLD,
      };
    });
  }, [callers, callCounts, statusCounts, outcomeCounts, fraudCounts]);

  const sortedStats = useMemo(() => {
    const sorted = [...callerStats].sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case "name":
          cmp = a.caller.name.localeCompare(b.caller.name, "ru");
          break;
        case "total":
          cmp = a.total - b.total;
          break;
        case "donePct":
          cmp = a.donePct - b.donePct;
          break;
        case "scriptCompleted":
          cmp = (a.outcomes[OUTCOME_SCRIPT_COMPLETED] ?? 0) - (b.outcomes[OUTCOME_SCRIPT_COMPLETED] ?? 0);
          break;
        case "step1Broken":
          cmp = (a.outcomes[OUTCOME_STEP1_BROKEN] ?? 0) - (b.outcomes[OUTCOME_STEP1_BROKEN] ?? 0);
          break;
        case "fraud":
          cmp = a.fraudCount - b.fraudCount;
          break;
        case "problem":
          cmp = a.problemRatio - b.problemRatio;
          break;
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
    return sorted;
  }, [callerStats, sortKey, sortDir]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir(key === "name" ? "asc" : "desc");
    }
  };

  const SortIcon = ({ column }: { column: SortKey }) => {
    if (sortKey !== column) return null;
    return sortDir === "asc" ? <ChevronUp className="ml-1 inline h-3 w-3" /> : <ChevronDown className="ml-1 inline h-3 w-3" />;
  };

  const kpi = useMemo(() => {
    const totalCalls = Object.values(callCounts).reduce((a, b) => a + b, 0);
    const totalDone = Object.values(statusCounts).reduce((sum, s) => sum + (s.done ?? 0), 0);
    const totalScriptCompleted = Object.values(outcomeCounts).reduce(
      (sum, o) => sum + (o[OUTCOME_SCRIPT_COMPLETED] ?? 0),
      0
    );
    const totalStep1Broken = Object.values(outcomeCounts).reduce((sum, o) => sum + (o[OUTCOME_STEP1_BROKEN] ?? 0), 0);
    // Backend buckets under "не проанализировано" via COALESCE(...outcome, 'не проанализировано')
    // — that only catches NULL analytics_json, not legacy rows with an
    // explicit empty-string outcome (CallOutcome's own "" variant, see
    // zvonari.ts) which bucket under "" instead. Count both.
    const totalUnanalyzed = Object.values(outcomeCounts).reduce(
      (sum, o) => sum + (o["не проанализировано"] ?? 0) + (o[""] ?? 0),
      0
    );
    const totalFraud = Object.values(fraudCounts).reduce((a, b) => a + b, 0);
    return { totalCalls, totalDone, totalScriptCompleted, totalStep1Broken, totalUnanalyzed, totalFraud };
  }, [callCounts, statusCounts, outcomeCounts, fraudCounts]);

  return (
    <div className="zvonari-theme min-h-screen bg-background">
      <div className="container mx-auto max-w-6xl px-4 py-8">
        <div className="mb-8 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div>
            <div className="mb-3">
              <Link href="/">
                <Button variant="outline" size="sm" className="transition-colors">
                  <ArrowLeft className="mr-2 h-4 w-4" />
                  Главная
                </Button>
              </Link>
            </div>
            <div className="flex items-center gap-3">
              <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Phone className="h-5 w-5" />
              </span>
              <h1 className="text-3xl font-bold tracking-tight md:text-4xl">Звонари</h1>
            </div>
            <p className="mt-2 text-muted-foreground">
              Звонки из OnlinePBX, транскрибация (Whisper локально) и аналитика по запросу через Hermes
            </p>
          </div>
        </div>

        <Card className="mb-6 border-border/70 shadow-sm">
          <CardHeader>
            <CardTitle className="text-lg">Синхронизация и анализ</CardTitle>
            <CardDescription>Период для загрузки CDR, транскрибации и классификации звонков</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap items-end gap-3">
              <div>
                <label className="mb-1 block text-sm text-muted-foreground">С</label>
                <Input
                  type="date"
                  value={from}
                  onChange={(e) => setFrom(e.target.value)}
                  className="w-40 transition-shadow focus-visible:ring-2 focus-visible:ring-ring"
                />
              </div>
              <div>
                <label className="mb-1 block text-sm text-muted-foreground">По</label>
                <Input
                  type="date"
                  value={to}
                  onChange={(e) => setTo(e.target.value)}
                  className="w-40 transition-shadow focus-visible:ring-2 focus-visible:ring-ring"
                />
              </div>
              <div className="flex gap-1 rounded-lg bg-muted p-1">
                {PERIOD_PRESETS.map((preset) => {
                  const active = from === preset.from() && to === preset.to();
                  return (
                    <Button
                      key={preset.label}
                      variant={active ? "default" : "ghost"}
                      size="sm"
                      className={`transition-colors ${active ? "" : "hover:bg-background"}`}
                      onClick={() => {
                        setFrom(preset.from());
                        setTo(preset.to());
                      }}
                    >
                      {preset.label}
                    </Button>
                  );
                })}
              </div>
              <div className="ml-auto flex flex-wrap gap-2">
                {syncing ? (
                  <Button
                    onClick={handlePauseResume}
                    disabled={pausing}
                    variant={paused ? "default" : "outline"}
                    className={`transition-transform active:scale-95 ${
                      paused ? "" : "border-warning text-warning hover:bg-warning/10 hover:text-warning"
                    }`}
                  >
                    {pausing ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : paused ? (
                      <Play className="mr-2 h-4 w-4" />
                    ) : (
                      <Pause className="mr-2 h-4 w-4" />
                    )}
                    {paused ? "Продолжить" : "Пауза"}
                  </Button>
                ) : (
                  <Button onClick={handleSync} className="transition-transform active:scale-95">
                    <RefreshCw className="mr-2 h-4 w-4" />
                    Синхронизировать
                  </Button>
                )}
                <Button
                  variant="outline"
                  onClick={handleAnalyze}
                  disabled={syncing}
                  className="transition-transform active:scale-95"
                >
                  <Sparkles className="mr-2 h-4 w-4" />
                  Анализировать
                </Button>
                <Button
                  variant="ghost"
                  onClick={handleRetryFailed}
                  disabled={syncing}
                  className="text-muted-foreground transition-transform hover:text-foreground active:scale-95"
                >
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Повторить неудачные
                </Button>
                <Button
                  variant="ghost"
                  onClick={handleRetranscribeGpu}
                  disabled={syncing}
                  title="Перетранскрибировать ВСЕ звонки периода (включая уже готовые) — использует GPU-бокс, если он настроен и доступен"
                  className="text-muted-foreground transition-transform hover:text-foreground active:scale-95"
                >
                  <Cpu className="mr-2 h-4 w-4" />
                  Перетранскрибировать на GPU
                </Button>
              </div>
            </div>
            {syncing && (
              <div className="mt-4">
                <div className="mb-1 flex justify-between text-xs text-muted-foreground">
                  <span className={`inline-flex items-center gap-1.5 ${paused ? "text-warning" : ""}`}>
                    {paused ? <Pause className="h-3 w-3" /> : <Loader2 className="h-3 w-3 animate-spin" />}
                    {paused ? "На паузе" : "Обработка звонков"}
                  </span>
                  {syncProgress && syncProgress.total > 0 && (
                    <span>
                      {syncProgress.processed} / {syncProgress.total} (
                      {Math.min(100, Math.round((syncProgress.processed / syncProgress.total) * 100))}%)
                    </span>
                  )}
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-muted">
                  <div
                    className={`h-full rounded-full transition-all duration-500 ${
                      paused ? "bg-warning" : "bg-primary"
                    } ${!syncProgress || syncProgress.total === 0 ? "w-1/3 animate-pulse" : ""}`}
                    style={
                      syncProgress && syncProgress.total > 0
                        ? { width: `${Math.min(100, (syncProgress.processed / syncProgress.total) * 100)}%` }
                        : undefined
                    }
                  />
                </div>
              </div>
            )}
            {syncError && (
              <p className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{syncError}</p>
            )}
            {syncMessage && !syncing && <p className="mt-3 text-sm text-muted-foreground">{syncMessage}</p>}
          </CardContent>
        </Card>

      {callers.length > 0 && (
        <div className="mb-6 grid grid-cols-2 gap-3 md:grid-cols-6">
          <KpiCard
            label="Всего звонков"
            value={String(kpi.totalCalls)}
            icon={<Phone className="h-5 w-5" />}
            accent="primary"
          />
          <KpiCard
            label="Готово"
            value={kpi.totalCalls > 0 ? `${Math.round((kpi.totalDone / kpi.totalCalls) * 100)}%` : "—"}
            hint={`${kpi.totalDone} из ${kpi.totalCalls}`}
            icon={<ListChecks className="h-5 w-5" />}
            accent="primary"
          />
          <KpiCard
            label="Скрипт пройден до шага 6"
            value={String(kpi.totalScriptCompleted)}
            icon={<CheckCircle2 className="h-5 w-5" />}
            accent="positive"
          />
          <KpiCard
            label="Срыв на шаге 1"
            value={String(kpi.totalStep1Broken)}
            icon={<XCircle className="h-5 w-5" />}
            accent="warning"
          />
          <KpiCard
            label="Автоответчик (фрод)"
            value={String(kpi.totalFraud)}
            hint={kpi.totalFraud > 0 ? "не сброшен вовремя" : undefined}
            icon={<ShieldAlert className="h-5 w-5" />}
            accent="negative"
          />
          <KpiCard
            label="Не проанализировано"
            value={String(kpi.totalUnanalyzed)}
            hint={kpi.totalUnanalyzed > 0 ? "нажмите «Анализировать»" : undefined}
            icon={<HelpCircle className="h-5 w-5" />}
            accent="neutral"
          />
        </div>
      )}

      {listError && (
        <p className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{listError}</p>
      )}
      {aggregatesError && (
        <p className="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{aggregatesError}</p>
      )}
      {loadingCallers ? (
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Загрузка...
        </div>
      ) : callers.length === 0 ? (
        <p className="text-muted-foreground">
          Звонарей пока нет — нажмите «Синхронизировать», чтобы подтянуть список из OnlinePBX.
        </p>
      ) : (
        <Card className="overflow-hidden border-border/70 shadow-sm">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-8" />
                <TableHead
                  className="cursor-pointer select-none transition-colors hover:text-foreground"
                  onClick={() => handleSort("name")}
                >
                  Звонарь
                  <SortIcon column="name" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("total")}
                >
                  Звонков
                  <SortIcon column="total" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("donePct")}
                >
                  Готово
                  <SortIcon column="donePct" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("scriptCompleted")}
                >
                  Скрипт до конца
                  <SortIcon column="scriptCompleted" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("step1Broken")}
                >
                  Срыв на шаге 1
                  <SortIcon column="step1Broken" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("fraud")}
                >
                  Фрод
                  <SortIcon column="fraud" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("problem")}
                >
                  Проблемные
                  <SortIcon column="problem" />
                </TableHead>
                <TableHead className="text-right">Действия</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedStats.map(({ caller, total, donePct, outcomes, fraudCount, problemRatio, isProblem }) => {
                const panel = panels[caller.id] || EMPTY_PANEL;
                const isExpanded = expandedId === caller.id;
                return (
                  <Fragment key={caller.id}>
                    <TableRow
                      role="button"
                      tabIndex={0}
                      className={`cursor-pointer transition-colors hover:bg-accent/60 ${
                        isExpanded ? "bg-accent/40" : ""
                      }`}
                      onClick={() => toggleExpand(caller.id)}
                      onKeyDown={(event) => {
                        if (event.target !== event.currentTarget) return
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault()
                          toggleExpand(caller.id)
                        }
                      }}
                    >
                      <TableCell>
                        <ChevronRight
                          className={`h-4 w-4 text-muted-foreground transition-transform duration-200 ${
                            isExpanded ? "rotate-90 text-primary" : ""
                          }`}
                        />
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{caller.name}</span>
                          {!caller.active && (
                            <Badge variant="secondary" className="text-xs">
                              неактивен
                            </Badge>
                          )}
                          {isProblem && (
                            <Badge variant="destructive" className="gap-1 text-xs">
                              <AlertTriangle className="h-3 w-3" />
                              требует внимания
                            </Badge>
                          )}
                        </div>
                        <div className="text-xs text-muted-foreground">внутр. номер {caller.pbx_extension}</div>
                      </TableCell>
                      <TableCell className="text-right">
                        <span className="inline-flex items-center gap-1">
                          <Phone className="h-3 w-3 text-muted-foreground" />
                          {total}
                        </span>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{total > 0 ? `${donePct}%` : "—"}</TableCell>
                      <TableCell className="text-right tabular-nums text-success">
                        {outcomes[OUTCOME_SCRIPT_COMPLETED] ?? 0}
                      </TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">
                        {outcomes[OUTCOME_STEP1_BROKEN] ?? 0}
                      </TableCell>
                      <TableCell className="text-right">
                        {fraudCount > 0 ? (
                          <Badge variant="destructive" className="gap-1 font-normal">
                            <ShieldAlert className="h-3 w-3" />
                            {fraudCount}
                          </Badge>
                        ) : (
                          <span className="text-muted-foreground">0</span>
                        )}
                      </TableCell>
                      <TableCell className={`text-right tabular-nums ${isProblem ? "text-destructive font-medium" : "text-muted-foreground"}`}>
                        {total > 0 ? `${Math.round(problemRatio * 100)}%` : "—"}
                      </TableCell>
                      <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleRequestReport(caller.id)}
                            disabled={panel.reportLoading}
                            className="transition-transform active:scale-95"
                          >
                            {panel.reportLoading ? (
                              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            ) : (
                              <FileBarChart className="mr-2 h-4 w-4" />
                            )}
                            {panel.reportLoading ? "Формирование..." : "Отчёт"}
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDownloadCsv(caller.id)}
                            className="transition-transform hover:text-primary active:scale-95"
                            title="Скачать CSV"
                            aria-label="Скачать CSV"
                          >
                            <Download className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                    {isExpanded && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell colSpan={8} className="bg-accent/20">
                          <div className="space-y-3 py-3">
                            {panel.distributionLoading ? (
                              <div className="flex items-center gap-2 rounded-lg border border-border/70 bg-card p-3 text-sm text-muted-foreground">
                                <Loader2 className="h-4 w-4 animate-spin" />
                                Загрузка распределения...
                              </div>
                            ) : panel.distribution ? (
                              <div className="rounded-lg border border-border/70 bg-card p-3">
                                <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
                                  <ListChecks className="h-4 w-4 text-primary" />
                                  Распределение звонков за период
                                </h4>
                                <DistributionBars distribution={panel.distribution} />
                              </div>
                            ) : null}

                            {panel.reportError && (
                              <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                                {panel.reportError}
                              </p>
                            )}
                            {panel.report && (
                              <div className="rounded-lg border border-border/70 bg-card p-3">
                                <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
                                  <FileBarChart className="h-4 w-4 text-primary" />
                                  Анализ за период
                                </h4>
                                <p className="whitespace-pre-wrap text-sm text-muted-foreground">
                                  {panel.report.summary_text}
                                </p>
                              </div>
                            )}

                            <div>
                              {panel.history === null ? (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleLoadHistory(caller.id)}
                                  disabled={panel.historyLoading}
                                  className="transition-transform active:scale-95"
                                >
                                  {panel.historyLoading ? (
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                  ) : (
                                    <History className="mr-2 h-4 w-4" />
                                  )}
                                  История отчётов
                                </Button>
                              ) : panel.history.length === 0 ? (
                                <p className="text-sm text-muted-foreground">Отчётов по этому звонарю ещё не было</p>
                              ) : (
                                <div className="rounded-lg border border-border/70 bg-card p-3">
                                  <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
                                    <History className="h-4 w-4 text-primary" />
                                    История отчётов ({panel.history.length})
                                  </h4>
                                  <div className="space-y-2">
                                    {panel.history.map((r) => (
                                      <details
                                        key={r.id}
                                        className="group rounded-md border border-border/70 p-2 transition-colors hover:bg-accent/40"
                                      >
                                        <summary className="cursor-pointer text-sm text-muted-foreground group-open:text-foreground">
                                          {new Date(r.requested_at).toLocaleString("ru-RU")} — {r.period_start} — {r.period_end}
                                        </summary>
                                        <p className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">
                                          {r.summary_text}
                                        </p>
                                      </details>
                                    ))}
                                  </div>
                                </div>
                              )}
                            </div>

                            <div>
                              {panel.calls === null ? (
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => handleLoadCalls(caller.id)}
                                  disabled={panel.callsLoading}
                                  className="transition-transform active:scale-95"
                                >
                                  {panel.callsLoading ? (
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                  ) : (
                                    <Phone className="mr-2 h-4 w-4" />
                                  )}
                                  Показать звонки
                                </Button>
                              ) : (
                                <div className="rounded-lg border border-border/70 bg-card p-3">
                                  <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
                                    <Phone className="h-4 w-4 text-primary" />
                                    Детализация по звонкам ({panel.calls.length})
                                  </h4>
                                  {panel.callsError && (
                                    <p className="mb-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                                      {panel.callsError}
                                    </p>
                                  )}
                                  <CallDetailList
                                    calls={panel.calls}
                                    retranscribingIds={retranscribingIds}
                                    onRetranscribe={(callId) => handleRetranscribeCall(caller.id, callId)}
                                  />
                                </div>
                              )}
                            </div>
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                );
              })}
            </TableBody>
          </Table>
        </Card>
      )}
      </div>
    </div>
  );
}
