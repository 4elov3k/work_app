"use client"
// Общие хелперы/компоненты для /zvonari (список) и /zvonari/[ext] (карточка
// звонаря) — вынесены при переходе на роут-на-звонаря вместо раскрывающейся
// строки (zvonari-ui-redesign.md, §2 и §5). Файл с префиксом "_" не
// участвует в роутинге Next.js.
import { Fragment, ReactNode, useEffect, useMemo, useRef, useState } from "react";
import {
  Mic,
  Loader2,
  Sparkles,
  ChevronRight,
  Search,
  ShieldAlert,
  CheckCircle2,
  XCircle,
  CircleDashed,
  CornerDownRight,
  Ban,
  HelpCircle,
  Target,
} from "lucide-react";

import { Caller, Call, CallAnalytics, StageStatus, PingResult } from "@/lib/api";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";

// ---------------------------------------------------------------------------
// Период, тренды
// ---------------------------------------------------------------------------

// Период (from/to) переживает перезагрузку страницы через localStorage —
// без этого при каждом обновлении фильтр молча слетал обратно на дефолтную
// неделю, теряя то, что реально смотрел человек.
export const PERIOD_STORAGE_KEY = "zvonari_period";

export function loadStoredPeriod(): { from: string; to: string } | null {
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

export function todayISO(offsetDays = 0): string {
  const d = new Date();
  d.setDate(d.getDate() + offsetDays);
  return d.toISOString().slice(0, 10);
}

// previousPeriod returns the period of the same length immediately
// preceding [from, to] — e.g. from=08-14/to=08-20 (7 days) -> 08-07/08-13 —
// so period-over-period KPI trends compare like-for-like windows rather than
// a fixed "last month" that could be a very different length.
export function previousPeriod(from: string, to: string): { from: string; to: string } {
  const fromDate = new Date(from + "T00:00:00Z");
  const toDate = new Date(to + "T00:00:00Z");
  const lengthDays = Math.max(1, Math.round((toDate.getTime() - fromDate.getTime()) / 86400000) + 1);
  const prevTo = new Date(fromDate.getTime() - 86400000);
  const prevFrom = new Date(prevTo.getTime() - (lengthDays - 1) * 86400000);
  return { from: prevFrom.toISOString().slice(0, 10), to: prevTo.toISOString().slice(0, 10) };
}

export function formatTrend(
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

export function formatPointTrend(
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

export const PERIOD_PRESETS: { label: string; from: () => string; to: () => string }[] = [
  { label: "Сегодня", from: () => todayISO(), to: () => todayISO() },
  { label: "Вчера", from: () => todayISO(-1), to: () => todayISO(-1) },
  { label: "Неделя", from: () => todayISO(-6), to: () => todayISO() },
  { label: "Прошлая неделя", from: () => todayISO(-13), to: () => todayISO(-7) },
  { label: "Месяц", from: () => todayISO(-29), to: () => todayISO() },
];

// computeKpi aggregates the 4 per-caller endpoint responses into the same
// summary numbers shown on the KPI cards — factored out (rather than inline
// in a useMemo) so the exact same logic can run once for the current period
// and once for the comparison period, guaranteeing the two numbers being
// diffed for a trend are always computed the same way.
export function computeKpi(
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

// ---------------------------------------------------------------------------
// Метки/константы
// ---------------------------------------------------------------------------

export const DIRECTION_LABELS: Record<string, string> = {
  outbound: "исходящий",
  inbound: "входящий",
  local: "внутренний",
};

// Причины ошибки транскрибации (error_kind, задача 6 zvonari-improvements.md)
// — терминальные не лечатся повтором, поэтому помечены отдельно для
// подсказки рядом со сводкой.
export const ERROR_KIND_LABELS: Record<string, string> = {
  no_recording: "нет записи",
  download_failed: "не скачалась запись",
  transcribe_failed: "ошибка транскрибации",
};
export const TERMINAL_ERROR_KINDS = new Set(["no_recording"]);

export const STATUS_LABELS: Record<string, string> = {
  done: "готово",
  failed: "ошибка",
  no_recording: "без записи",
  pending: "в очереди",
  transcribing: "обрабатывается",
};

export const STATUS_ORDER = ["done", "transcribing", "pending", "failed", "no_recording"];

// Итог "Скрипт пройден до шага 6" — полное прохождение обязательных шагов
// скрипта (регламент IQ-200 v1.2, §11) — и "Срыв на шаге 1" — самый ранний
// возможный срыв — используются как headline-метрики звонаря.
export const OUTCOME_SCRIPT_COMPLETED = "Скрипт пройден до шага 6";
export const OUTCOME_STEP1_BROKEN = "Срыв на шаге 1";

export const CALL_TYPE_LABELS: Record<string, string> = {
  "технический": "технический",
  "содержательный": "содержательный",
  "недостаточно_данных": "недостаточно данных",
};

export const STEP_LABELS: Record<string, string> = {
  step1: "Шаг 1 — выход на ЛПР",
  step2: "Шаг 2 — знакомство",
  step3: "Шаг 3 — первичная потребность",
  step4: "Шаг 4 — вилка времени",
  step5: "Шаг 5 — предмет встречи",
  step6: "Шаг 6 — фиксация встречи",
};

export const STEP_KEYS: (keyof NonNullable<CallAnalytics["steps"]>)[] = [
  "step1",
  "step2",
  "step3",
  "step4",
  "step5",
  "step6",
];

// Порог, начиная с которого звонарь помечается как "требует внимания" —
// доля ошибок/без записи от всех его звонков за период. Игнорируем совсем
// маленькие выборки (<5 звонков), чтобы один неудачный звонок не красил
// звонаря с 1-2 звонками в красный без статистической значимости.
export const PROBLEM_RATIO_THRESHOLD = 0.3;
export const PROBLEM_MIN_CALLS = 5;

export function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

// ---------------------------------------------------------------------------
// Дорожка скрипта — фирменный элемент (zvonari-ui-redesign.md §3).
// Один компонент, три размера: h-1.5 в строке звонка (вердикт этого
// конкретного звонка), h-2.5 в шапке звонаря и h-4 в KPI периода (агрегат —
// доля звонков, дошедших до каждого шага). Единственный визуальный элемент,
// который "заслуживает узнаваемости" — весь остальной интерфейс тихий.
// ---------------------------------------------------------------------------

export type SegmentTone = "ok" | "partial" | "fail" | "na";

export interface TrackSegment {
  tone: SegmentTone;
  tooltip: string;
}

const SEGMENT_TONE_CLASSES: Record<SegmentTone, string> = {
  ok: "bg-success",
  partial: "bg-warning",
  fail: "bg-destructive",
  na: "bg-na",
};

const TRACK_SIZE_CLASSES = { sm: "h-1.5", md: "h-2.5", lg: "h-4" } as const;

// useTooltip — настоящий Tooltip только для md/lg (единицы штук на страницу);
// в плотной таблице (sm, до 50 строк × 6 сегментов) — нативный title, чтобы
// не плодить сотни Radix-инстансов без ощутимой пользы для UX.
export function ScriptTrack({
  segments,
  size = "sm",
  useTooltip = false,
  className = "",
}: {
  segments: TrackSegment[];
  size?: keyof typeof TRACK_SIZE_CLASSES;
  useTooltip?: boolean;
  className?: string;
}) {
  return (
    <div className={`flex items-center gap-0.5 ${className}`}>
      {segments.map((seg, i) =>
        useTooltip ? (
          <Tooltip key={i}>
            <TooltipTrigger asChild>
              <span
                className={`w-3.5 flex-1 rounded-sm ${TRACK_SIZE_CLASSES[size]} ${SEGMENT_TONE_CLASSES[seg.tone]}`}
              />
            </TooltipTrigger>
            <TooltipContent>{seg.tooltip}</TooltipContent>
          </Tooltip>
        ) : (
          <span
            key={i}
            title={seg.tooltip}
            className={`w-2.5 flex-1 rounded-sm ${TRACK_SIZE_CLASSES[size]} ${SEGMENT_TONE_CLASSES[seg.tone]}`}
          />
        )
      )}
    </div>
  );
}

// Статус шага (регламент §3) -> тон сегмента дорожки. step4 — бинарный
// ("Использован"/нет), не входит в общую шкалу выполнен/частично/не выполнен.
function stepTone(status: string | undefined, stepKey: string): SegmentTone {
  if (stepKey === "step4") return status === "Использован" ? "ok" : "na";
  switch (status) {
    case "Выполнен":
      return "ok";
    case "Частично":
    case "Выполнен вне последовательности":
      return "partial";
    case "Не выполнен":
      return "fail";
    default:
      return "na"; // "Корректная остановка" / "Не применим" / нет данных
  }
}

// Сегменты дорожки для ОДНОГО звонка (строка таблицы, size="sm") — точные
// данные из analytics_json.steps, без каких-либо допущений.
export function stepSegmentsFromAnalytics(steps?: CallAnalytics["steps"]): TrackSegment[] {
  return STEP_KEYS.map((key) => {
    const status = steps?.[key]?.status;
    return { tone: stepTone(status, key), tooltip: `${STEP_LABELS[key]}: ${status || "нет данных"}` };
  });
}

// OUTCOME_DEPTH сопоставляет каждому из 13 исходов регламента, до какого
// шага дошёл звонок — восстановлено из порядка ROADMAP_MAIN_PATH ниже
// (сам регламент такого числа не хранит). Нужно, чтобы агрегатная дорожка
// (шапка звонаря/KPI периода) считалась по уже загруженным счётчикам
// исходов, а не требовала тянуть analytics_json.steps каждого звонка только
// ради одной сводной полоски.
const OUTCOME_DEPTH: Record<string, number> = {
  [OUTCOME_STEP1_BROKEN]: 0,
  "Шаг 1 выполнен": 1,
  "Срыв на шаге 2": 1,
  "Шаг 2 выполнен": 2,
  "Шаг 3 выполнен вне последовательности": 3,
  "Шаг 3 выполнен": 3,
  "Согласован конкретный повторный контакт": 3,
  "Встреча согласована, шаг 5 не выполнен": 4,
  [OUTCOME_SCRIPT_COMPLETED]: 6,
};

// Сегменты агрегатной дорожки (шапка звонаря / KPI периода, size="md"/"lg") —
// доля звонков периода, дошедших хотя бы до каждого шага. Знаменатель — все
// звонки периода (не только с распознанным исходом), чтобы звонарь с
// большинством ещё не проанализированных звонков не казался прошедшим
// скрипт лучше, чем на самом деле.
export function trackSegmentsFromDistribution(distribution: Record<string, number>): TrackSegment[] {
  const total = Object.values(distribution).reduce((a, b) => a + b, 0);
  const reached = [0, 0, 0, 0, 0, 0];
  for (const [outcome, count] of Object.entries(distribution)) {
    const depth = OUTCOME_DEPTH[outcome];
    if (depth === undefined || count <= 0) continue;
    for (let step = 1; step <= depth; step++) reached[step - 1] += count;
  }
  return STEP_KEYS.map((key, i) => {
    const fraction = total > 0 ? reached[i] / total : 0;
    const tone: SegmentTone = fraction >= 0.5 ? "ok" : fraction > 0 ? "partial" : "na";
    return { tone, tooltip: `${STEP_LABELS[key]}: дошли ${Math.round(fraction * 100)}%` };
  });
}

// ---------------------------------------------------------------------------
// Разбор по шагам конкретного звонка (раскрытая строка)
// ---------------------------------------------------------------------------

function stepStatusMeta(status: string | undefined): { icon: ReactNode; className: string } {
  switch (status) {
    case "Выполнен":
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

export function StepBreakdown({ analytics }: { analytics: CallAnalytics }) {
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

// ---------------------------------------------------------------------------
// Итог звонка (бейдж)
// ---------------------------------------------------------------------------

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

export function OutcomeBadge({ outcome }: { outcome?: string }) {
  if (!outcome) return <span className="text-xs text-muted-foreground">не проанализировано</span>;
  return (
    <Badge variant="outline" className={`whitespace-normal text-left font-normal ${OUTCOME_TONE_CLASSES[outcomeTone(outcome)]}`}>
      {outcome}
    </Badge>
  );
}

// ---------------------------------------------------------------------------
// Дорожная карта скрипта — вертикальная воронка (таб "Обзор"). Заменяет
// прежнюю горизонтальную цепочку чипов со стрелками (zvonari-ui-redesign.md
// §2): скрипт — это отсев, вертикальные полосы убывающей длины показывают
// его сразу, без чтения 13 чисел подряд.
// ---------------------------------------------------------------------------

export interface RoadmapStage {
  value: string;
  label: string;
  tone: "positive" | "negative" | "neutral" | "muted";
}

// Порядок — путь через скрипт слева направо/сверху вниз (см. комментарий у
// OUTCOME_DEPTH выше). "Корректно выявлено отсутствие потребности" и фрод —
// намеренно не в основном пути (см. ROADMAP_OTHER_PATH) — это не веха "как
// далеко дошли", а отдельные исходы.
export const ROADMAP_MAIN_PATH: RoadmapStage[] = [
  { value: OUTCOME_STEP1_BROKEN, label: "Шаг 1: срыв", tone: "negative" },
  { value: "Шаг 1 выполнен", label: "Шаг 1 пройден", tone: "neutral" },
  { value: "Срыв на шаге 2", label: "Шаг 2: срыв", tone: "negative" },
  { value: "Шаг 2 выполнен", label: "Шаг 2 пройден", tone: "neutral" },
  { value: "Шаг 3 выполнен вне последовательности", label: "Шаг 3 вне очереди", tone: "muted" },
  { value: "Шаг 3 выполнен", label: "Шаг 3 пройден", tone: "neutral" },
  { value: "Согласован конкретный повторный контакт", label: "Повторный контакт", tone: "positive" },
  { value: "Встреча согласована, шаг 5 не выполнен", label: "Встреча, шаг 5 не закрыт", tone: "negative" },
  { value: OUTCOME_SCRIPT_COMPLETED, label: "Шаг 6: скрипт пройден", tone: "positive" },
];

export const ROADMAP_OTHER_PATH: RoadmapStage[] = [
  { value: "Технический / содержательный диалог не состоялся", label: "Диалог не состоялся", tone: "muted" },
  { value: "Корректная ранняя остановка", label: "Ранняя остановка", tone: "muted" },
  { value: "Недостаточно данных для оценки", label: "Недостаточно данных", tone: "muted" },
];

export const ROADMAP_BAR_TONE_CLASSES: Record<RoadmapStage["tone"], string> = {
  positive: "bg-success",
  negative: "bg-destructive",
  neutral: "bg-primary",
  muted: "bg-na",
};

// CallOutcome uses "" to mean "not yet analyzed" (see zvonari.ts), which
// collides with the roadmap filter's own "no filter selected" sentinel —
// this separate value lets a stage represent "unanalyzed" instead of it
// being unreachable.
export const UNANALYZED_OUTCOME = "__unanalyzed__";
// fraud_suspected is a boolean field on analytics_json, not one of the 13
// outcome values — this sentinel lets the funnel's "Фрод" row drive the same
// single filter state as the outcome stages even though it filters a
// different field.
export const UNANALYZED_FRAUD_FILTER = "__fraud__";

function FunnelBar({
  label,
  count,
  maxCount,
  toneClass,
  selected,
  onClick,
}: {
  label: string;
  count: number;
  maxCount: number;
  toneClass: string;
  selected: boolean;
  onClick: () => void;
}) {
  const widthPct = maxCount > 0 ? Math.max(count > 0 ? 4 : 0, Math.round((count / maxCount) * 100)) : 0;
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={count === 0}
      className={`group flex w-full items-center gap-2 rounded-md py-1 text-left transition-colors ${
        count === 0 ? "cursor-default opacity-50" : "hover:bg-accent/60"
      } ${selected ? "bg-accent" : ""}`}
    >
      <span className="w-40 shrink-0 truncate text-xs text-muted-foreground group-hover:text-foreground">{label}</span>
      <span className="h-4 flex-1 overflow-hidden rounded-sm bg-muted">
        <span className={`block h-full rounded-sm transition-all ${toneClass}`} style={{ width: `${widthPct}%` }} />
      </span>
      <span className="w-8 shrink-0 text-right font-mono text-xs tabular-nums">{count}</span>
    </button>
  );
}

export function ScriptFunnel({
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
  const unanalyzedCount = (distribution["не проанализировано"] ?? 0) + (distribution[""] ?? 0);
  const noNeedCount = distribution["Корректно выявлено отсутствие потребности"] ?? 0;
  const toggle = (value: string) => onSelect(selected === value ? "" : value);
  const maxCount = Math.max(1, ...ROADMAP_MAIN_PATH.map((s) => distribution[s.value] ?? 0));

  // Стадии с нулём не рисуются вовсе — вместо них одна строка "Шаги N–M:
  // никто не дошёл" (zvonari-ui-redesign.md §2), а не пустой прочерк на
  // каждую.
  const visibleMain = ROADMAP_MAIN_PATH.filter((s) => (distribution[s.value] ?? 0) > 0);
  const zeroMain = ROADMAP_MAIN_PATH.filter((s) => (distribution[s.value] ?? 0) === 0);

  return (
    <div className="space-y-3">
      <div className="space-y-0.5">
        {visibleMain.map((stage) => (
          <FunnelBar
            key={stage.value}
            label={stage.label}
            count={distribution[stage.value] ?? 0}
            maxCount={maxCount}
            toneClass={ROADMAP_BAR_TONE_CLASSES[stage.tone]}
            selected={selected === stage.value}
            onClick={() => toggle(stage.value)}
          />
        ))}
        {zeroMain.length > 0 && (
          <p className="py-1 text-xs text-muted-foreground">
            {zeroMain.length === ROADMAP_MAIN_PATH.length
              ? "Ни один звонок пока не дошёл ни до одного шага скрипта."
              : `${zeroMain.map((s) => s.label).join(", ")}: никто не дошёл.`}
          </p>
        )}
      </div>
      <div className="space-y-0.5 border-t border-border/60 pt-2">
        <p className="mb-1 text-xs font-medium text-muted-foreground">Вне скрипта</p>
        {noNeedCount > 0 && (
          <FunnelBar
            label="Нет потребности (корректно)"
            count={noNeedCount}
            maxCount={maxCount}
            toneClass="bg-success"
            selected={selected === "Корректно выявлено отсутствие потребности"}
            onClick={() => toggle("Корректно выявлено отсутствие потребности")}
          />
        )}
        {fraudCount > 0 && (
          <FunnelBar
            label="Фрод (автоответчик)"
            count={fraudCount}
            maxCount={maxCount}
            toneClass="bg-destructive"
            selected={selected === UNANALYZED_FRAUD_FILTER}
            onClick={() => toggle(UNANALYZED_FRAUD_FILTER)}
          />
        )}
        {ROADMAP_OTHER_PATH.filter((s) => (distribution[s.value] ?? 0) > 0).map((stage) => (
          <FunnelBar
            key={stage.value}
            label={stage.label}
            count={distribution[stage.value] ?? 0}
            maxCount={maxCount}
            toneClass={ROADMAP_BAR_TONE_CLASSES[stage.tone]}
            selected={selected === stage.value}
            onClick={() => toggle(stage.value)}
          />
        ))}
        {unanalyzedCount > 0 && (
          <FunnelBar
            label="Не проанализировано"
            count={unanalyzedCount}
            maxCount={maxCount}
            toneClass="bg-na"
            selected={selected === UNANALYZED_OUTCOME}
            onClick={() => toggle(UNANALYZED_OUTCOME)}
          />
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Список звонков (таб "Звонки")
// ---------------------------------------------------------------------------

export interface CallFilters {
  status: string;
  callType: string;
  direction: string;
  search: string;
}

// search передаётся снаружи (не локальный стейт), чтобы попадать в URL —
// см. onSearchChange. status/callType/direction в URL не идут и остаются
// локальным состоянием этого компонента.
export type LocalCallFilters = Omit<CallFilters, "search">;
export const EMPTY_LOCAL_FILTERS: LocalCallFilters = { status: "", callType: "", direction: "" };

export function filterCalls(calls: Call[], filters: CallFilters, roadmapFilter: string, errorKindFilter: string): Call[] {
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

export const CALLS_PAGE_SIZE = 50;

export function CallDetailList({
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
  errorKindFilter: string;
  onClearErrorKindFilter: () => void;
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
  const pageCount = Math.max(1, Math.ceil(filtered.length / CALLS_PAGE_SIZE));
  const currentPage = Math.min(page, pageCount - 1);
  const paginated = useMemo(
    () => filtered.slice(currentPage * CALLS_PAGE_SIZE, (currentPage + 1) * CALLS_PAGE_SIZE),
    [filtered, currentPage]
  );
  // page живёт в родителе (участвует в URL) — сброс на смену
  // фильтров/списка через эффект, не через render-phase adjustment (тот
  // приём годится только для собственного state компонента).
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
      <div className="flex flex-wrap items-center gap-2 rounded-md border border-border/70 bg-muted/40 p-2">
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
          Найдено {filtered.length} из {calls.length}
        </span>
      </div>

      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">Ничего не найдено по этим фильтрам</p>
      ) : (
        <div className="overflow-hidden rounded-md border border-border/70">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-6" />
                <TableHead>Время</TableHead>
                <TableHead>Длит.</TableHead>
                <TableHead>
                  <span className="inline-flex items-center gap-1">
                    Прогресс по скрипту
                    <span
                      title="Зелёный — выполнен, жёлтый — частично/вне очереди, красный — не выполнен, серый — н/д"
                      className="cursor-help text-muted-foreground/70"
                    >
                      <HelpCircle className="h-3 w-3" />
                    </span>
                  </span>
                </TableHead>
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
                      className={`h-11 cursor-pointer transition-colors hover:bg-accent/50 ${isExpanded ? "bg-accent/30" : ""} ${
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
                          className={`h-4 w-4 text-muted-foreground transition-transform duration-150 ${
                            isExpanded ? "rotate-90 text-primary" : ""
                          }`}
                        />
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-sm">
                        <div className="font-mono tabular-nums">{new Date(call.started_at).toLocaleString("ru-RU")}</div>
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <Badge variant="outline" className="px-1 py-0 text-[10px] font-normal">
                            {DIRECTION_LABELS[call.direction] || call.direction}
                          </Badge>
                          {notDone && (
                            <span className="text-warning">{STATUS_LABELS[call.transcript_status] || call.transcript_status}</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="whitespace-nowrap font-mono text-sm tabular-nums text-muted-foreground">
                        {formatDuration(call.duration_sec)}
                      </TableCell>
                      <TableCell className="w-28">
                        <ScriptTrack segments={stepSegmentsFromAnalytics(analytics?.steps)} size="sm" />
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
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-muted-foreground transition-transform hover:text-foreground active:scale-95"
                              disabled={isRetranscribing}
                              aria-label="Перетранскрибировать"
                              onClick={() => onRetranscribe(call.id)}
                            >
                              {isRetranscribing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Mic className="h-3.5 w-3.5" />}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>Перетранскрибировать</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-muted-foreground transition-transform hover:text-foreground active:scale-95"
                              disabled={isAnalyzing || !call.transcript_text}
                              aria-label="Переанализировать"
                              onClick={() => onAnalyze(call.id)}
                            >
                              {isAnalyzing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>Переанализировать (пересчитать исход и фрод по текущему транскрипту)</TooltipContent>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                    {isExpanded && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell colSpan={6} className="bg-accent/10">
                          <div className="space-y-2 py-1">
                            {analytics?.steps && <StepBreakdown analytics={analytics} />}
                            {call.hangup_cause && !analytics?.steps && (
                              <p className="text-xs text-muted-foreground">Причина завершения: {call.hangup_cause}</p>
                            )}
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

// ---------------------------------------------------------------------------
// KPI-плитки (список звонарей)
// ---------------------------------------------------------------------------

export const KPI_ACCENTS = {
  neutral: "border-l-border",
  primary: "border-l-primary",
  positive: "border-l-success",
  negative: "border-l-destructive",
  warning: "border-l-warning",
} as const;

export const TREND_TONE_CLASSES = {
  positive: "text-success",
  negative: "text-destructive",
  neutral: "text-muted-foreground",
} as const;

export function KpiCard({
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
    <Card className={`border-l-4 ${KPI_ACCENTS[accent]}`}>
      <CardContent className="pt-5">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <div className="font-mono text-2xl font-semibold tabular-nums tracking-tight">{value}</div>
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

// ---------------------------------------------------------------------------
// Состояние конвейера (задача 1/2 zvonari-improvements.md) — оставлено в
// прежнем виде, три независимые полосы.
// ---------------------------------------------------------------------------

export const STAGE_ORDER = ["sync", "transcribe", "analyze"];
export const STAGE_LABELS: Record<string, string> = {
  sync: "Синк",
  transcribe: "Транскрибация",
  analyze: "Анализ",
};

export function StageBar({ label, stage }: { label: string; stage?: StageStatus }) {
  const state = stage?.state ?? "idle";
  const barClass =
    state === "failed" ? "bg-destructive" : state === "running" ? "bg-primary" : state === "done" ? "bg-success" : "bg-na";
  const total = stage?.total ?? 0;
  const done = stage?.done ?? 0;
  const pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : state === "running" ? undefined : 0;
  const stateLabel = state === "running" ? "выполняется" : state === "done" ? "готово" : state === "failed" ? "ошибка" : "не запускалась";
  return (
    <div className="min-w-0 flex-1">
      <div className="mb-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="flex items-center gap-1.5 truncate">
          {state === "running" && <Loader2 className="h-3 w-3 shrink-0 animate-spin text-primary" />}
          {label}
        </span>
        <span className="shrink-0 font-mono tabular-nums">{total > 0 ? `${done}/${total}` : stateLabel}</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          className={`h-full rounded-full transition-all duration-500 ${barClass} ${pct === undefined ? "w-1/3 animate-pulse" : ""}`}
          style={pct !== undefined ? { width: `${pct}%` } : undefined}
        />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Точки здоровья внешних сервисов (задача 7 zvonari-improvements.md)
// ---------------------------------------------------------------------------

export function HealthDot({ label, ping }: { label: string; ping?: PingResult }) {
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

// ---------------------------------------------------------------------------
// Статистика звонаря (строка списка)
// ---------------------------------------------------------------------------

export interface CallerStats {
  caller: Caller;
  total: number;
  done: number;
  donePct: number;
  outcomes: Record<string, number>;
  fraudCount: number;
  problemRatio: number;
  isProblem: boolean;
}

export type SortKey = "name" | "total" | "donePct" | "scriptCompleted" | "step1Broken" | "fraud" | "problem";
export const SORT_KEYS: SortKey[] = ["name", "total", "donePct", "scriptCompleted", "step1Broken", "fraud", "problem"];
