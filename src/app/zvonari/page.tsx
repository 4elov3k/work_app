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

import { zvonariAPI, Caller, CallerReport, Call, CallAnalytics, ApiError, RetranscribePreview, HealthStatus, PingResult } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

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

// previousPeriod returns the period of the same length immediately
// preceding [from, to] — e.g. from=08-14/to=08-20 (7 days) -> 08-07/08-13 —
// so period-over-period KPI trends compare like-for-like windows rather than
// a fixed "last month" that could be a very different length.
function previousPeriod(from: string, to: string): { from: string; to: string } {
  const fromDate = new Date(from + "T00:00:00Z");
  const toDate = new Date(to + "T00:00:00Z");
  const lengthDays = Math.max(1, Math.round((toDate.getTime() - fromDate.getTime()) / 86400000) + 1);
  const prevTo = new Date(fromDate.getTime() - 86400000);
  const prevFrom = new Date(prevTo.getTime() - (lengthDays - 1) * 86400000);
  return { from: prevFrom.toISOString().slice(0, 10), to: prevTo.toISOString().slice(0, 10) };
}

// formatTrend compares current vs previous and returns display text plus a
// tone: goodDirection says which direction ("up"/"down") is an improvement
// for this particular metric (e.g. up is good for "done", down is good for
// "не проанализировано"); pass "neutral" for metrics where neither
// direction is inherently good/bad (e.g. raw call volume).
function formatTrend(
  current: number,
  previous: number,
  goodDirection: "up" | "down" | "neutral"
): { text: string; tone: "positive" | "negative" | "neutral" } {
  if (previous === 0) {
    if (current === 0) return { text: "без изменений к пред. периоду", tone: "neutral" };
    const tone = goodDirection === "neutral" ? "neutral" : goodDirection === "up" ? "positive" : "negative";
    return { text: `+${current}, не было в пред. периоде`, tone };
  }
  const pct = Math.round(((current - previous) / previous) * 100);
  if (pct === 0) return { text: "без изменений к пред. периоду", tone: "neutral" };
  const direction: "up" | "down" = pct > 0 ? "up" : "down";
  const tone =
    goodDirection === "neutral" ? "neutral" : direction === goodDirection ? "positive" : "negative";
  return { text: `${pct > 0 ? "+" : ""}${pct}% к пред. периоду`, tone };
}

// formatPointTrend is formatTrend's counterpart for a value that is itself
// already a percentage (e.g. "% готово") — the delta is shown in
// percentage points ("+7 п.п."), not a relative percent-of-percent change,
// which would conflate "50% -> 60%" with a confusing "+20%".
function formatPointTrend(
  current: number | null,
  previous: number | null,
  goodDirection: "up" | "down" | "neutral"
): { text: string; tone: "positive" | "negative" | "neutral" } | undefined {
  if (current === null || previous === null) return undefined;
  const diff = Math.round(current - previous);
  if (diff === 0) return { text: "без изменений к пред. периоду", tone: "neutral" };
  const direction: "up" | "down" = diff > 0 ? "up" : "down";
  const tone =
    goodDirection === "neutral" ? "neutral" : direction === goodDirection ? "positive" : "negative";
  return { text: `${diff > 0 ? "+" : ""}${diff} п.п. к пред. периоду`, tone };
}

// computeKpi aggregates the 4 per-caller endpoint responses into the same
// summary numbers shown on the KPI cards — factored out (rather than inline
// in a useMemo) so the exact same logic can run once for the current period
// and once for the comparison period, guaranteeing the two numbers being
// diffed for a trend are always computed the same way.
function computeKpi(
  callCounts: Record<string, number>,
  statusCounts: Record<string, Record<string, number>>,
  outcomeCounts: Record<string, Record<string, number>>,
  fraudCounts: Record<string, number>
) {
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

// Причины ошибки транскрибации (error_kind, задача 6 zvonari-improvements.md)
// — терминальные не лечатся повтором, поэтому помечены отдельно для
// подсказки рядом со сводкой.
const ERROR_KIND_LABELS: Record<string, string> = {
  no_recording: "нет записи",
  download_failed: "не скачалась запись",
  transcribe_failed: "ошибка транскрибации",
};
const TERMINAL_ERROR_KINDS = new Set(["no_recording"]);

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

// The IQ-200 v1.2 rubric's closed list of 13 outcome values (see
// baza_znaniy_ocenka_zvonkov_po_skriptu_IQ200_v1.2.md, section 11), ordered
// here to read as a left-to-right journey through the script rather than
// the alphabetical/arbitrary order a plain distribution table would give.
// "Корректно выявлено отсутствие потребности" and fraud are deliberately
// NOT in this path: the former is a valid terminal state reached only after
// the step-3 diagnosis (not a "how far did they get" milestone), and fraud
// is an orthogonal boolean axis, not one of the outcome values at all — see
// ExtractFraudSuspected/FraudCounts on the backend. Both render as separate
// callout boxes beside the main path instead.
interface RoadmapStage {
  value: string;
  label: string;
  tone: "positive" | "negative" | "neutral" | "muted";
}

const ROADMAP_MAIN_PATH: RoadmapStage[] = [
  { value: "Срыв на шаге 1", label: "Шаг 1: срыв", tone: "negative" },
  { value: "Шаг 1 выполнен", label: "Шаг 1 пройден", tone: "neutral" },
  { value: "Срыв на шаге 2", label: "Шаг 2: срыв", tone: "negative" },
  { value: "Шаг 2 выполнен", label: "Шаг 2 пройден", tone: "neutral" },
  { value: "Шаг 3 выполнен вне последовательности", label: "Шаг 3 вне очереди", tone: "muted" },
  { value: "Шаг 3 выполнен", label: "Шаг 3 пройден", tone: "neutral" },
  { value: "Согласован конкретный повторный контакт", label: "Повторный контакт", tone: "positive" },
  { value: "Встреча согласована, шаг 5 не выполнен", label: "Встреча, шаг 5 не закрыт", tone: "negative" },
  { value: OUTCOME_SCRIPT_COMPLETED, label: "Шаг 6: скрипт пройден", tone: "positive" },
];

const ROADMAP_OTHER_PATH: RoadmapStage[] = [
  { value: "Технический / содержательный диалог не состоялся", label: "Диалог не состоялся", tone: "muted" },
  { value: "Корректная ранняя остановка", label: "Ранняя остановка", tone: "muted" },
  { value: "Недостаточно данных для оценки", label: "Недостаточно данных", tone: "muted" },
];

const ROADMAP_TONE_CLASSES: Record<RoadmapStage["tone"], string> = {
  positive: "border-success/40 bg-success/10 text-success",
  negative: "border-destructive/40 bg-destructive/10 text-destructive",
  neutral: "border-primary/40 bg-primary/10 text-primary",
  muted: "border-border bg-muted/40 text-muted-foreground",
};

function RoadmapStageChip({
  label,
  count,
  tone,
  selected,
  onClick,
}: {
  label: string;
  count: number;
  tone: RoadmapStage["tone"];
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex shrink-0 flex-col items-center gap-0.5 rounded-md border px-2.5 py-1.5 text-center transition-all active:scale-95 ${ROADMAP_TONE_CLASSES[tone]} ${
        selected ? "ring-2 ring-ring" : count === 0 ? "opacity-40 hover:opacity-70" : "hover:opacity-80"
      }`}
      title={label}
    >
      <span className="text-base font-bold leading-none tabular-nums">{count}</span>
      <span className="max-w-[6.5rem] whitespace-normal text-[11px] leading-tight">{label}</span>
    </button>
  );
}

// ScriptRoadmap replaces the old plain bar-chart distribution: instead of
// an unordered "here are 13 bars", it lays the same counts out in script
// order (see ROADMAP_MAIN_PATH) so it reads like a funnel — where calls
// actually stopped — and doubles as the outcome filter (click a stage to
// filter the call list to it, click again to clear) instead of a separate
// dropdown.
function ScriptRoadmap({
  distribution,
  fraudCount,
  selected,
  onSelect,
}: {
  distribution: Record<string, number>;
  fraudCount: number;
  selected: string;
  onSelect: (value: string) => void;
}) {
  const total = Object.values(distribution).reduce((a, b) => a + b, 0);
  if (total === 0 && fraudCount === 0) {
    return <p className="text-sm text-muted-foreground">Нет звонков за этот период</p>;
  }
  const unanalyzedCount = distribution["не проанализировано"] ?? 0;
  const noNeedCount = distribution["Корректно выявлено отсутствие потребности"] ?? 0;
  const toggle = (value: string) => onSelect(selected === value ? "" : value);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-1.5">
        {ROADMAP_MAIN_PATH.map((stage, i) => (
          <Fragment key={stage.value}>
            {i > 0 && <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" />}
            <RoadmapStageChip
              label={stage.label}
              count={distribution[stage.value] ?? 0}
              tone={stage.tone}
              selected={selected === stage.value}
              onClick={() => toggle(stage.value)}
            />
          </Fragment>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-1.5 border-t border-border/60 pt-2">
        <span className="mr-1 text-xs text-muted-foreground">Отдельно:</span>
        <RoadmapStageChip
          label="Правильно скинут (нет потребности)"
          count={noNeedCount}
          tone="positive"
          selected={selected === "Корректно выявлено отсутствие потребности"}
          onClick={() => toggle("Корректно выявлено отсутствие потребности")}
        />
        <RoadmapStageChip
          label="Фрод (автоответчик)"
          count={fraudCount}
          tone="negative"
          selected={selected === UNANALYZED_FRAUD_FILTER}
          onClick={() => toggle(UNANALYZED_FRAUD_FILTER)}
        />
        {ROADMAP_OTHER_PATH.map((stage) => (
          <RoadmapStageChip
            key={stage.value}
            label={stage.label}
            count={distribution[stage.value] ?? 0}
            tone={stage.tone}
            selected={selected === stage.value}
            onClick={() => toggle(stage.value)}
          />
        ))}
        {unanalyzedCount > 0 && (
          <RoadmapStageChip
            label="Не проанализировано"
            count={unanalyzedCount}
            tone="muted"
            selected={selected === UNANALYZED_OUTCOME}
            onClick={() => toggle(UNANALYZED_OUTCOME)}
          />
        )}
      </div>
    </div>
  );
}

interface CallFilters {
  status: string;
  callType: string;
  direction: string;
  search: string;
}

// search живёт в родительском компоненте (не здесь), чтобы попадать в URL —
// см. onSearchChange в CallDetailList и синхронизацию query-параметров в
// ZvonariPage (задача 5, zvonari-improvements.md). status/callType/direction
// в URL не идут, поэтому остаются локальным состоянием этого компонента.
type LocalCallFilters = Omit<CallFilters, "search">;
const EMPTY_LOCAL_FILTERS: LocalCallFilters = { status: "", callType: "", direction: "" };

// CallOutcome uses "" to mean "not yet analyzed" (see zvonari.ts), which
// collides with the roadmap filter's own "no filter selected" sentinel —
// this separate value lets a stage represent "unanalyzed" instead of it
// being unreachable.
const UNANALYZED_OUTCOME = "__unanalyzed__";
// fraud_suspected is a boolean field on analytics_json, not one of the 13
// outcome values (see ExtractFraudSuspected on the backend) — this sentinel
// lets the roadmap's "Фрод" chip drive the same single filter state as the
// outcome stages even though it filters a different field.
const UNANALYZED_FRAUD_FILTER = "__fraud__";

// roadmapFilter is a single value covering both outcome stages and the two
// side callouts (unanalyzed, fraud) — see ScriptRoadmap, which replaced the
// separate outcome dropdown and "только фрод" toggle this list used to have.
function filterCalls(calls: Call[], filters: CallFilters, roadmapFilter: string, errorKindFilter: string): Call[] {
  return calls.filter((call) => {
    if (filters.status && call.transcript_status !== filters.status) return false;
    if (filters.callType && (call.analytics_json?.call_type || "") !== filters.callType) return false;
    if (roadmapFilter === UNANALYZED_OUTCOME) {
      if (call.analytics_json?.outcome) return false;
    } else if (roadmapFilter === UNANALYZED_FRAUD_FILTER) {
      if (!call.analytics_json?.fraud_suspected) return false;
    } else if (roadmapFilter && (call.analytics_json?.outcome || "") !== roadmapFilter) {
      return false;
    }
    if (errorKindFilter && call.error_kind !== errorKindFilter) return false;
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
  analyzingIds,
  onAnalyze,
  roadmapFilter,
  onClearRoadmapFilter,
  errorKindFilter,
  onClearErrorKindFilter,
  search,
  onSearchChange,
  page,
  onPageChange,
}: {
  calls: Call[];
  retranscribingIds: Set<string>;
  onRetranscribe: (callId: string) => void;
  analyzingIds: Set<string>;
  onAnalyze: (callId: string) => void;
  roadmapFilter: string;
  onClearRoadmapFilter: () => void;
  // Причина ошибки (error_kind) — задача 6, выбирается из сводки под
  // блоком синхронизации, живёт в родителе так же, как roadmapFilter.
  errorKindFilter: string;
  onClearErrorKindFilter: () => void;
  // search/page живут в родительском ZvonariPage, а не здесь — попадают в
  // URL (query-параметры q/page), чтобы ссылка на отфильтрованный список
  // открывалась у другого человека в том же виде (задача 5).
  search: string;
  onSearchChange: (value: string) => void;
  page: number;
  onPageChange: (page: number) => void;
}) {
  const [localFilters, setLocalFilters] = useState<LocalCallFilters>(EMPTY_LOCAL_FILTERS);
  const [expandedCallId, setExpandedCallId] = useState<string | null>(null);
  const filters: CallFilters = useMemo(() => ({ ...localFilters, search }), [localFilters, search]);
  const filtered = useMemo(
    () => filterCalls(calls, filters, roadmapFilter, errorKindFilter),
    [calls, filters, roadmapFilter, errorKindFilter]
  );
  // Some periods return hundreds of calls (each with a full transcript) —
  // rendering them all into one unvirtualized table is genuinely slow, so
  // paginate client-side rather than pulling in a virtualization library.
  const pageCount = Math.max(1, Math.ceil(filtered.length / CALLS_PAGE_SIZE));
  const currentPage = Math.min(page, pageCount - 1);
  const paginated = useMemo(
    () => filtered.slice(currentPage * CALLS_PAGE_SIZE, (currentPage + 1) * CALLS_PAGE_SIZE),
    [filtered, currentPage]
  );
  // Reset to page 1 when the filters or the underlying call list change.
  // page now lives in the parent (see onPageChange) so it can round-trip
  // through the URL, so this can no longer be the render-phase
  // local-state-adjustment trick the previous version used (that pattern is
  // only sound for a component's own state, not a parent's) — an effect
  // triggers the reset instead; `currentPage`'s Math.min clamp above already
  // keeps the visible page in range for the one render before it fires.
  const resetTrackedOn = useRef({ filters, calls, roadmapFilter, errorKindFilter });
  useEffect(() => {
    if (
      resetTrackedOn.current.filters !== filters ||
      resetTrackedOn.current.calls !== calls ||
      resetTrackedOn.current.roadmapFilter !== roadmapFilter ||
      resetTrackedOn.current.errorKindFilter !== errorKindFilter
    ) {
      resetTrackedOn.current = { filters, calls, roadmapFilter, errorKindFilter };
      onPageChange(0);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters, calls, roadmapFilter, errorKindFilter]);

  const callTypes = useMemo(() => {
    const set = new Set<string>();
    calls.forEach((c) => c.analytics_json?.call_type && set.add(c.analytics_json.call_type));
    return Array.from(set).sort();
  }, [calls]);

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
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="h-8 w-56 bg-card pl-7 text-sm"
          />
        </div>
        <select
          value={localFilters.status}
          onChange={(e) => setLocalFilters((f) => ({ ...f, status: e.target.value }))}
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
          value={localFilters.callType}
          onChange={(e) => setLocalFilters((f) => ({ ...f, callType: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все типы звонков</option>
          {callTypes.map((c) => (
            <option key={c} value={c}>
              {CALL_TYPE_LABELS[c] || c}
            </option>
          ))}
        </select>
        <select
          value={localFilters.direction}
          onChange={(e) => setLocalFilters((f) => ({ ...f, direction: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все направления</option>
          {Object.entries(DIRECTION_LABELS).map(([key, label]) => (
            <option key={key} value={key}>
              {label}
            </option>
          ))}
        </select>
        {errorKindFilter && (
          <Badge variant="outline" className="gap-1 border-destructive/40 bg-destructive/10 text-destructive">
            Причина: {ERROR_KIND_LABELS[errorKindFilter] || errorKindFilter}
            <button type="button" onClick={onClearErrorKindFilter} aria-label="Сбросить фильтр по причине" className="ml-0.5">
              ×
            </button>
          </Badge>
        )}
        {(localFilters.status || localFilters.callType || localFilters.direction || search || roadmapFilter || errorKindFilter) && (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => {
              setLocalFilters(EMPTY_LOCAL_FILTERS);
              onSearchChange("");
              onClearRoadmapFilter();
              onClearErrorKindFilter();
            }}
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
                const isAnalyzing = analyzingIds.has(call.id);
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
                      <TableCell onClick={(e) => e.stopPropagation()} className="whitespace-nowrap">
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
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2 transition-transform active:scale-95"
                          disabled={isAnalyzing || !call.transcript_text}
                          title="Переанализировать (пересчитать исход и фрод по текущему транскрипту)"
                          aria-label="Переанализировать"
                          onClick={() => onAnalyze(call.id)}
                        >
                          {isAnalyzing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
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
                  onClick={() => onPageChange(currentPage - 1)}
                >
                  Назад
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2"
                  disabled={currentPage >= pageCount - 1}
                  onClick={() => onPageChange(currentPage + 1)}
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
const SORT_KEYS: SortKey[] = ["name", "total", "donePct", "scriptCompleted", "step1Broken", "fraud", "problem"];

const KPI_ACCENTS = {
  neutral: "border-l-border",
  primary: "border-l-primary",
  positive: "border-l-success",
  negative: "border-l-destructive",
  warning: "border-l-warning",
} as const;

const TREND_TONE_CLASSES = {
  positive: "text-success",
  negative: "text-destructive",
  neutral: "text-muted-foreground",
} as const;

function KpiCard({
  label,
  value,
  hint,
  icon,
  accent = "neutral",
  trend,
}: {
  label: string;
  value: string;
  hint?: string;
  icon?: ReactNode;
  accent?: keyof typeof KPI_ACCENTS;
  trend?: { text: string; tone: keyof typeof TREND_TONE_CLASSES };
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
        {trend && <div className={`mt-1 text-xs ${TREND_TONE_CLASSES[trend.tone]}`}>{trend.text}</div>}
      </CardContent>
    </Card>
  );
}

// Точка-индикатор здоровья одного внешнего сервиса (задача 7,
// zvonari-improvements.md) — зелёная/красная/серая вместо гадания по тексту
// ошибки синхронизации, отвечает ли вообще сервис. title вместо
// Tooltip-компонента — в проекте нет shadcn tooltip.tsx, а нативный
// title уже используется как тултип везде на этой странице (см. кнопки
// перетранскрибации/переанализа на строке звонка).
function HealthDot({ label, ping }: { label: string; ping?: PingResult }) {
  const className = !ping || !ping.configured
    ? "bg-muted-foreground/30"
    : ping.ok
    ? "bg-success"
    : "bg-destructive";
  const title = !ping
    ? `${label}: проверяем...`
    : !ping.configured
    ? `${label}: не настроен`
    : ping.ok
    ? `${label}: в порядке`
    : `${label}: недоступен${ping.error ? ` (${ping.error})` : ""}`;
  return <span title={title} className={`inline-block h-2.5 w-2.5 rounded-full ${className}`} />;
}

export default function ZvonariPage() {
  const [callers, setCallers] = useState<Caller[]>([]);
  const [loadingCallers, setLoadingCallers] = useState(true);
  const [listError, setListError] = useState("");
  const [aggregatesError, setAggregatesError] = useState("");

  // Здоровье внешних сервисов (задача 7) — опрашиваем раз в 30с (совпадает с
  // кешем на бэкенде, так что чаще смысла не имеет); первая проверка сразу
  // при монтировании, не дожидаясь первого тика интервала.
  const [health, setHealth] = useState<HealthStatus | null>(null);
  useEffect(() => {
    let cancelled = false;
    const check = () => {
      zvonariAPI
        .getHealth()
        .then((response) => {
          if (!cancelled) setHealth(response.data);
        })
        .catch((err) => console.error("Health check failed:", err));
    };
    check();
    const interval = setInterval(check, 30000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

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

  // Same 4 aggregates for the immediately-preceding period of equal length,
  // used to show "+N% к пред. периоду" trend text on the KPI cards below —
  // see previousPeriod()/formatTrend(). prevLoaded stays false while any of
  // the 4 requests is in flight or failed, so trends are hidden rather than
  // shown next to a half-loaded/stale comparison.
  const [prevCallCounts, setPrevCallCounts] = useState<Record<string, number>>({});
  const [prevStatusCounts, setPrevStatusCounts] = useState<Record<string, Record<string, number>>>({});
  const [prevOutcomeCounts, setPrevOutcomeCounts] = useState<Record<string, Record<string, number>>>({});
  const [prevFraudCounts, setPrevFraudCounts] = useState<Record<string, number>>({});
  const [prevLoaded, setPrevLoaded] = useState(false);

  const [panels, setPanels] = useState<Record<string, CallerPanelState>>({});
  const [expandedId, setExpandedId] = useState<string | null>(null);
  // Drives both the ScriptRoadmap's highlighted stage and the calls list's
  // filtering — only one caller panel can be expanded at a time, so a single
  // value (not per-caller) is enough; reset whenever the expanded caller
  // changes (see toggleExpand) so a stage picked for one звонарь doesn't
  // leak into the next one's call list.
  const [roadmapFilter, setRoadmapFilter] = useState("");
  // Живут здесь (не внутри CallDetailList), чтобы попадать в URL как q/page
  // — задача 5, zvonari-improvements.md: ссылку на отфильтрованный список
  // звонков конкретного звонаря можно переслать, и она откроется в том же
  // виде. Сбрасываются при смене звонаря/периода вместе с roadmapFilter.
  const [callSearch, setCallSearch] = useState("");
  const [callPage, setCallPage] = useState(0);
  // Причина ошибки, выбранная в сводке под блоком синхронизации (задача 6) —
  // тот же паттерн единственного значения на всю страницу, что у
  // roadmapFilter, потому что раскрыт всегда только один звонарь.
  const [errorKindFilter, setErrorKindFilter] = useState("");
  const [errorBreakdown, setErrorBreakdown] = useState<Record<string, number>>({});
  const [retryIncludeTerminal, setRetryIncludeTerminal] = useState(false);
  const [retranscribingIds, setRetranscribingIds] = useState<Set<string>>(new Set());
  const [analyzingIds, setAnalyzingIds] = useState<Set<string>>(new Set());

  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  // Задача 5 (zvonari-improvements.md): период/сортировка/раскрытый звонарь/
  // фильтр по исходу/поиск/страница переживают перезагрузку и попадают в
  // ссылку, которую можно переслать. Восстанавливаем из query-параметров
  // один раз после монтирования (после localStorage выше — URL специфичнее
  // и должен победить, если он вообще есть); toggleExpand определён ниже в
  // этом же компоненте, но замыкание разрешается в момент вызова эффекта
  // (после монтирования), когда он уже присвоен — не в момент объявления.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const urlFrom = params.get("from");
    const urlTo = params.get("to");
    if (urlFrom && urlTo) {
      setFrom(urlFrom);
      setTo(urlTo);
    }
    const urlSort = params.get("sort");
    if (urlSort && (SORT_KEYS as string[]).includes(urlSort)) {
      setSortKey(urlSort as SortKey);
    }
    const urlDir = params.get("dir");
    if (urlDir === "asc" || urlDir === "desc") setSortDir(urlDir);
    const urlZvonar = params.get("zvonar");
    // Открываем звонаря так же, как клик по строке (подтягивает дорожку
    // скрипта) — список звонков всё равно требует отдельного клика
    // "Показать звонки" даже для человека, открывшего страницу напрямую,
    // так что ссылка это поведение не меняет.
    if (urlZvonar) toggleExpand(urlZvonar);
    const urlOutcome = params.get("outcome");
    if (urlOutcome) setRoadmapFilter(urlOutcome);
    const urlQ = params.get("q");
    if (urlQ) setCallSearch(urlQ);
    const urlPage = params.get("page");
    const parsedPage = urlPage ? parseInt(urlPage, 10) : NaN;
    if (!isNaN(parsedPage) && parsedPage >= 0) setCallPage(parsedPage);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Пишем состояние обратно в URL через replaceState (не через Next-роутер,
  // чтобы не триггерить навигацию/повторный рендер страницы) — история
  // браузера не засоряется, но перезагрузка или пересланная ссылка
  // воспроизводят ровно то, что было на экране.
  useEffect(() => {
    const params = new URLSearchParams();
    params.set("from", from);
    params.set("to", to);
    params.set("sort", sortKey);
    params.set("dir", sortDir);
    if (expandedId) params.set("zvonar", expandedId);
    if (roadmapFilter) params.set("outcome", roadmapFilter);
    if (callSearch) params.set("q", callSearch);
    if (callPage > 0) params.set("page", String(callPage));
    const newUrl = `${window.location.pathname}?${params.toString()}`;
    window.history.replaceState(null, "", newUrl);
  }, [from, to, sortKey, sortDir, expandedId, roadmapFilter, callSearch, callPage]);

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
    zvonariAPI
      .getErrorBreakdown(periodFrom, periodTo)
      .then((response) => setErrorBreakdown(response.data || {}))
      .catch((err) => console.error("Failed to load error breakdown:", err));
  };

  // Same 4 requests as loadAggregates, for the comparison period — kept
  // separate (rather than parameterizing loadAggregates with the setters)
  // so a slow/failed previous-period fetch can never overwrite the current
  // period's numbers, and so it fails silently (hides the trend, no error
  // banner) instead of competing with aggregatesError for the same banner.
  const loadPreviousAggregates = (periodFrom: string, periodTo: string) => {
    setPrevLoaded(false);
    Promise.all([
      zvonariAPI.getCallCounts(periodFrom, periodTo),
      zvonariAPI.getStatusCounts(periodFrom, periodTo),
      zvonariAPI.getOutcomeCounts(periodFrom, periodTo),
      zvonariAPI.getFraudCounts(periodFrom, periodTo),
    ])
      .then(([callsRes, statusRes, outcomeRes, fraudRes]) => {
        setPrevCallCounts(callsRes.data || {});
        setPrevStatusCounts(statusRes.data || {});
        setPrevOutcomeCounts(outcomeRes.data || {});
        setPrevFraudCounts(fraudRes.data || {});
        setPrevLoaded(true);
      })
      .catch((err) => {
        console.error("Failed to load previous-period statistics for trends:", err);
        setPrevLoaded(false);
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
    const { from: prevFrom, to: prevTo } = previousPeriod(from, to);
    loadPreviousAggregates(prevFrom, prevTo);
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
      () => zvonariAPI.retryFailed(period.from, period.to, retryIncludeTerminal),
      retryIncludeTerminal
        ? "Повтор запущен (включая «нет записи»), это может занять несколько минут..."
        : "Повтор запущен, это может занять несколько минут..."
    );
  // "Перетранскрибировать на GPU" — самая дорогая операция в системе (может
  // пересчитать тысячи звонков за один клик), поэтому вместо прямого запуска
  // сперва показываем диалог подтверждения с реальными числами — см.
  // openGpuDialog/confirmGpuRetranscribe и zvonari-improvements.md, задача 4.
  const [gpuDialogOpen, setGpuDialogOpen] = useState(false);
  const [gpuOnlyCpu, setGpuOnlyCpu] = useState(true);
  const [gpuPreview, setGpuPreview] = useState<RetranscribePreview | null>(null);
  const [gpuPreviewLoading, setGpuPreviewLoading] = useState(false);

  const loadGpuPreview = (onlyCpu: boolean) => {
    setGpuPreviewLoading(true);
    zvonariAPI
      .getRetranscribePreview(period.from, period.to, onlyCpu)
      .then((response) => setGpuPreview(response.data))
      .catch((err) => {
        console.error("Failed to load retranscribe preview:", err);
        setGpuPreview(null);
      })
      .finally(() => setGpuPreviewLoading(false));
  };

  const openGpuDialog = () => {
    setGpuDialogOpen(true);
    loadGpuPreview(gpuOnlyCpu);
  };

  const toggleGpuOnlyCpu = (value: boolean) => {
    setGpuOnlyCpu(value);
    loadGpuPreview(value);
  };

  const confirmGpuRetranscribe = () => {
    setGpuDialogOpen(false);
    runBackgroundJob(
      () => zvonariAPI.retranscribeAllGpu(period.from, period.to, gpuOnlyCpu),
      gpuOnlyCpu
        ? "Перетранскрибация запущена — звонки, ещё не снятые на GPU, будут пересняты (GPU, если доступен)..."
        : "Перетранскрибация запущена — ВСЕ звонки периода будут пересняты заново (GPU, если доступен)..."
    );
  };
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
    setRoadmapFilter("");
    setErrorKindFilter("");
    setCallSearch("");
    setCallPage(0);
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

  const handleAnalyzeCall = async (callerId: string, callId: string) => {
    setAnalyzingIds((current) => new Set(current).add(callId));
    updatePanel(callerId, { callsError: "" });
    try {
      const response = await zvonariAPI.analyzeCall(callId);
      const updated = response.data;
      setPanels((current) => {
        const panel = current[callerId];
        if (!panel?.calls) return current;
        return { ...current, [callerId]: { ...panel, calls: panel.calls.map((c) => (c.id === callId ? updated : c)) } };
      });
    } catch (err) {
      console.error("Analyze failed:", err);
      updatePanel(callerId, {
        callsError: err instanceof Error ? err.message : "Не удалось переанализировать звонок",
      });
    } finally {
      setAnalyzingIds((current) => {
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

  const kpi = useMemo(
    () => computeKpi(callCounts, statusCounts, outcomeCounts, fraudCounts),
    [callCounts, statusCounts, outcomeCounts, fraudCounts]
  );
  const prevKpi = useMemo(
    () => computeKpi(prevCallCounts, prevStatusCounts, prevOutcomeCounts, prevFraudCounts),
    [prevCallCounts, prevStatusCounts, prevOutcomeCounts, prevFraudCounts]
  );
  const trend = (current: number, previous: number, goodDirection: "up" | "down" | "neutral") =>
    prevLoaded ? formatTrend(current, previous, goodDirection) : undefined;

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
              <span className="flex items-center gap-1.5 rounded-full border border-border/70 bg-card px-2 py-1">
                <HealthDot label="Транскрибация (CPU)" ping={health?.transcribe_cpu} />
                <HealthDot label="Транскрибация (GPU)" ping={health?.transcribe_gpu} />
                <HealthDot label="Аналитика (Hermes)" ping={health?.analytics} />
              </span>
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
                <label className="flex items-center gap-1.5 self-center text-xs text-muted-foreground" title="Без записи повтор не помогает — OnlinePBX уже подтвердил, что записи нет">
                  <input
                    type="checkbox"
                    checked={retryIncludeTerminal}
                    onChange={(e) => setRetryIncludeTerminal(e.target.checked)}
                    className="h-3.5 w-3.5 rounded border-input"
                  />
                  включая «нет записи»
                </label>
                <Button
                  variant="outline"
                  onClick={openGpuDialog}
                  disabled={syncing}
                  title="Пересчитать транскрипты на GPU — самая дорогая операция, требует подтверждения"
                  className="border-destructive/50 text-destructive transition-transform hover:bg-destructive/10 hover:text-destructive active:scale-95"
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
            {Object.keys(errorBreakdown).length > 0 && (
              <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-border/60 pt-3">
                <span className="text-xs text-muted-foreground">Ошибки по причинам:</span>
                {Object.entries(errorBreakdown)
                  .sort((a, b) => b[1] - a[1])
                  .map(([kind, count]) => (
                    <button
                      key={kind}
                      type="button"
                      onClick={() => setErrorKindFilter((current) => (current === kind ? "" : kind))}
                      title={TERMINAL_ERROR_KINDS.has(kind) ? "Повтором не лечится" : "Клик — отфильтровать список звонков"}
                      className={`rounded-md border px-2 py-1 text-xs transition-colors ${
                        errorKindFilter === kind
                          ? "border-destructive bg-destructive/15 text-destructive"
                          : "border-border bg-destructive/5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                      }`}
                    >
                      {ERROR_KIND_LABELS[kind] || kind} — {count}
                    </button>
                  ))}
              </div>
            )}
          </CardContent>
        </Card>

      <Dialog open={gpuDialogOpen} onOpenChange={setGpuDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <Cpu className="h-5 w-5" />
              Перетранскрибировать на GPU
            </DialogTitle>
            <DialogDescription>
              Пересчитывает транскрипты заново — самая дорогая операция в системе. Проверьте объём перед запуском.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 text-sm">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={gpuOnlyCpu}
                onChange={(e) => toggleGpuOnlyCpu(e.target.checked)}
                className="h-4 w-4 rounded border-input"
              />
              Только те, что делались на CPU
            </label>
            {gpuPreviewLoading ? (
              <p className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Считаем...
              </p>
            ) : gpuPreview ? (
              <div className="rounded-md bg-muted/50 p-3">
                <p>
                  Будет пересчитано{" "}
                  <span className="font-semibold tabular-nums">
                    {gpuOnlyCpu ? gpuPreview.only_cpu_total : gpuPreview.total}
                  </span>{" "}
                  звонков за {period.from} – {period.to}
                  {gpuOnlyCpu && gpuPreview.already_gpu > 0 && (
                    <> (из {gpuPreview.total}, {gpuPreview.already_gpu} уже на GPU и будут пропущены)</>
                  )}
                  .
                </p>
                <p className="mt-1 text-muted-foreground">
                  Примерная длительность (по CPU-скорости, GPU обычно быстрее): ~
                  {gpuPreview.estimated_minutes < 1 ? "меньше минуты" : `${Math.ceil(gpuPreview.estimated_minutes)} мин`}
                </p>
              </div>
            ) : (
              <p className="text-muted-foreground">Не удалось посчитать оценку — можно всё равно продолжить.</p>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setGpuDialogOpen(false)}>
              Отмена
            </Button>
            <Button variant="destructive" onClick={confirmGpuRetranscribe}>
              Запустить перетранскрибацию
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {callers.length > 0 && (
        <div className="mb-6 grid grid-cols-2 gap-3 md:grid-cols-6">
          <KpiCard
            label="Всего звонков"
            value={String(kpi.totalCalls)}
            icon={<Phone className="h-5 w-5" />}
            accent="primary"
            trend={trend(kpi.totalCalls, prevKpi.totalCalls, "neutral")}
          />
          <KpiCard
            label="Готово"
            value={kpi.totalCalls > 0 ? `${Math.round((kpi.totalDone / kpi.totalCalls) * 100)}%` : "—"}
            hint={`${kpi.totalDone} из ${kpi.totalCalls}`}
            icon={<ListChecks className="h-5 w-5" />}
            accent="primary"
            trend={
              prevLoaded
                ? formatPointTrend(
                    kpi.totalCalls > 0 ? (kpi.totalDone / kpi.totalCalls) * 100 : null,
                    prevKpi.totalCalls > 0 ? (prevKpi.totalDone / prevKpi.totalCalls) * 100 : null,
                    "up"
                  )
                : undefined
            }
          />
          <KpiCard
            label="Скрипт пройден до шага 6"
            value={String(kpi.totalScriptCompleted)}
            icon={<CheckCircle2 className="h-5 w-5" />}
            accent="positive"
            trend={trend(kpi.totalScriptCompleted, prevKpi.totalScriptCompleted, "up")}
          />
          <KpiCard
            label="Срыв на шаге 1"
            value={String(kpi.totalStep1Broken)}
            icon={<XCircle className="h-5 w-5" />}
            accent="warning"
            trend={trend(kpi.totalStep1Broken, prevKpi.totalStep1Broken, "down")}
          />
          <KpiCard
            label="Автоответчик (фрод)"
            value={String(kpi.totalFraud)}
            hint={kpi.totalFraud > 0 ? "не сброшен вовремя" : undefined}
            icon={<ShieldAlert className="h-5 w-5" />}
            accent="negative"
            trend={trend(kpi.totalFraud, prevKpi.totalFraud, "down")}
          />
          <KpiCard
            label="Не проанализировано"
            value={String(kpi.totalUnanalyzed)}
            hint={kpi.totalUnanalyzed > 0 ? "нажмите «Анализировать»" : undefined}
            icon={<HelpCircle className="h-5 w-5" />}
            accent="neutral"
            trend={trend(kpi.totalUnanalyzed, prevKpi.totalUnanalyzed, "down")}
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
                                  Дорожная карта скрипта за период
                                </h4>
                                <ScriptRoadmap
                                  distribution={panel.distribution}
                                  fraudCount={fraudCount}
                                  selected={roadmapFilter}
                                  onSelect={setRoadmapFilter}
                                />
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
                                    search={callSearch}
                                    onSearchChange={setCallSearch}
                                    page={callPage}
                                    onPageChange={setCallPage}
                                    retranscribingIds={retranscribingIds}
                                    onRetranscribe={(callId) => handleRetranscribeCall(caller.id, callId)}
                                    analyzingIds={analyzingIds}
                                    onAnalyze={(callId) => handleAnalyzeCall(caller.id, callId)}
                                    roadmapFilter={roadmapFilter}
                                    onClearRoadmapFilter={() => setRoadmapFilter("")}
                                    errorKindFilter={errorKindFilter}
                                    onClearErrorKindFilter={() => setErrorKindFilter("")}
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
