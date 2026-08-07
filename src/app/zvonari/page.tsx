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
  ThumbsUp,
  ThumbsDown,
  ShieldAlert,
  HelpCircle,
} from "lucide-react";

import { zvonariAPI, Caller, CallerReport, Call, ApiError } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";

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

// Глобальная категория звонка — не сброшенный вовремя автоответчик
// (подозрение на АФК-накрутку времени на линии), в отличие от outcome,
// который классифицирует только реальные разговоры. Должно совпадать с
// FRAUD_CATEGORY в backend/internal/zvonari/service.go и FRAUD_CATEGORY в
// hermes/services/call_analytics_server.py.
const FRAUD_CATEGORY = "не сброшенный автоответчик (фрод)";

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
  category: string;
  outcome: string;
  direction: string;
  search: string;
}

const EMPTY_FILTERS: CallFilters = { status: "", category: "", outcome: "", direction: "", search: "" };

function filterCalls(calls: Call[], filters: CallFilters): Call[] {
  return calls.filter((call) => {
    if (filters.status && call.transcript_status !== filters.status) return false;
    if (filters.category && (call.analytics_json?.category || "") !== filters.category) return false;
    if (filters.outcome && (call.analytics_json?.outcome || "") !== filters.outcome) return false;
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
  const filtered = useMemo(() => filterCalls(calls, filters), [calls, filters]);

  const outcomes = useMemo(() => {
    const set = new Set<string>();
    calls.forEach((c) => c.analytics_json?.outcome && set.add(c.analytics_json.outcome));
    return Array.from(set).sort();
  }, [calls]);

  const categories = useMemo(() => {
    const set = new Set<string>();
    calls.forEach((c) => c.analytics_json?.category && set.add(c.analytics_json.category));
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
          value={filters.category}
          onChange={(e) => setFilters((f) => ({ ...f, category: e.target.value }))}
          className={selectClass}
        >
          <option value="">Все типы звонков</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        {categories.includes(FRAUD_CATEGORY) && (
          <Button
            variant={filters.category === FRAUD_CATEGORY ? "destructive" : "outline"}
            size="sm"
            className="h-8 gap-1.5 bg-card transition-transform active:scale-95"
            onClick={() =>
              setFilters((f) => ({ ...f, category: f.category === FRAUD_CATEGORY ? "" : FRAUD_CATEGORY }))
            }
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
        {(filters.status || filters.category || filters.outcome || filters.direction || filters.search) && (
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
        <div className="space-y-2">
          {filtered.map((call) => {
            const isRetranscribing = retranscribingIds.has(call.id);
            const isFraud = call.analytics_json?.category === FRAUD_CATEGORY;
            return (
              <details
                key={call.id}
                className={`group rounded-md border p-2 transition-colors hover:bg-accent/40 ${
                  isFraud ? "border-l-4 border-l-destructive" : ""
                }`}
              >
                <summary className="flex cursor-pointer flex-wrap items-center gap-2 text-sm">
                  <span className="text-muted-foreground">{new Date(call.started_at).toLocaleString("ru-RU")}</span>
                  <Badge variant="outline">{DIRECTION_LABELS[call.direction] || call.direction}</Badge>
                  <span className="text-muted-foreground">{formatDuration(call.duration_sec)}</span>
                  {call.analytics_json?.category && (
                    <Badge variant={isFraud ? "destructive" : "outline"} className="gap-1">
                      {isFraud && <ShieldAlert className="h-3 w-3" />}
                      {call.analytics_json.category}
                    </Badge>
                  )}
                  {call.analytics_json?.outcome && <Badge variant="secondary">{call.analytics_json.outcome}</Badge>}
                  <span
                    className={`text-xs ${call.transcript_status === "done" ? "text-muted-foreground" : "text-warning"}`}
                  >
                    {STATUS_LABELS[call.transcript_status] || call.transcript_status}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="ml-auto h-7 px-2 transition-transform active:scale-95"
                    disabled={isRetranscribing}
                    onClick={(e) => {
                      e.preventDefault();
                      onRetranscribe(call.id);
                    }}
                  >
                    {isRetranscribing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Mic className="h-3.5 w-3.5" />}
                    <span className="ml-1">Транскрибировать</span>
                  </Button>
                </summary>
                <p className="mt-2 whitespace-pre-wrap rounded-md bg-muted/50 p-2 text-sm text-muted-foreground">
                  {call.transcript_text || "Транскрипт недоступен"}
                </p>
              </details>
            );
          })}
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

type SortKey = "name" | "total" | "donePct" | "заинтересован" | "отказ" | "fraud" | "problem";

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
          <div>
            <div className="text-2xl font-bold tracking-tight">{value}</div>
            <div className="text-sm text-muted-foreground">{label}</div>
          </div>
          {icon && <span className="text-muted-foreground/60">{icon}</span>}
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

  const [from, setFrom] = useState(todayISO(-6));
  const [to, setTo] = useState(todayISO());

  const [syncing, setSyncing] = useState(false);
  const [syncError, setSyncError] = useState("");
  const [syncMessage, setSyncMessage] = useState("");
  const [syncProgress, setSyncProgress] = useState<{ total: number; processed: number } | null>(null);

  const [callCounts, setCallCounts] = useState<Record<string, number>>({});
  const [statusCounts, setStatusCounts] = useState<Record<string, Record<string, number>>>({});
  const [outcomeCounts, setOutcomeCounts] = useState<Record<string, Record<string, number>>>({});
  const [categoryCounts, setCategoryCounts] = useState<Record<string, Record<string, number>>>({});

  const [panels, setPanels] = useState<Record<string, CallerPanelState>>({});
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [retranscribingIds, setRetranscribingIds] = useState<Set<string>>(new Set());

  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  const period = useMemo(() => ({ from, to }), [from, to]);

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
    zvonariAPI
      .getCallCounts(periodFrom, periodTo)
      .then((response) => setCallCounts(response.data || {}))
      .catch((err) => console.error("Failed to load call counts:", err));
    zvonariAPI
      .getStatusCounts(periodFrom, periodTo)
      .then((response) => setStatusCounts(response.data || {}))
      .catch((err) => console.error("Failed to load status counts:", err));
    zvonariAPI
      .getOutcomeCounts(periodFrom, periodTo)
      .then((response) => setOutcomeCounts(response.data || {}))
      .catch((err) => console.error("Failed to load outcome counts:", err));
    zvonariAPI
      .getCategoryCounts(periodFrom, periodTo)
      .then((response) => setCategoryCounts(response.data || {}))
      .catch((err) => console.error("Failed to load category counts:", err));
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
    const interval = setInterval(async () => {
      try {
        const response = await zvonariAPI.getSyncStatus();
        const status = response.data;
        if (status.total_to_process) {
          setSyncProgress({ total: status.total_to_process, processed: status.processed ?? 0 });
        }
        refreshLiveData();
        if (!status.running) {
          clearInterval(interval);
          setSyncing(false);
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
        clearInterval(interval);
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
          setSyncMessage("Задача уже выполняется...");
          pollSyncStatus();
        }
      })
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    loadAggregates(from, to);
  }, [from, to]);

  const runBackgroundJob = async (
    starter: () => Promise<{ data: { status: string } }>,
    startMessage: string
  ) => {
    setSyncing(true);
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
  const handleAnalyze = () =>
    runBackgroundJob(
      () => zvonariAPI.analyzeCalls(period.from, period.to),
      "Анализ запущен — классифицируем готовые транскрипты..."
    );

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

  const handleDownloadCsv = (callerId: string) => {
    const url = zvonariAPI.exportCallsCsvUrl(callerId, period.from, period.to);
    window.location.href = url;
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
      const categories = categoryCounts[caller.id] ?? {};
      const done = statuses.done ?? 0;
      const problems = (statuses.failed ?? 0) + (statuses.no_recording ?? 0);
      const problemRatio = total > 0 ? problems / total : 0;
      return {
        caller,
        total,
        done,
        donePct: total > 0 ? Math.round((done / total) * 100) : 0,
        outcomes,
        fraudCount: categories[FRAUD_CATEGORY] ?? 0,
        problemRatio,
        isProblem: total >= PROBLEM_MIN_CALLS && problemRatio >= PROBLEM_RATIO_THRESHOLD,
      };
    });
  }, [callers, callCounts, statusCounts, outcomeCounts, categoryCounts]);

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
        case "заинтересован":
        case "отказ":
          cmp = (a.outcomes[sortKey] ?? 0) - (b.outcomes[sortKey] ?? 0);
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
    const totalInterested = Object.values(outcomeCounts).reduce((sum, o) => sum + (o["заинтересован"] ?? 0), 0);
    const totalRefused = Object.values(outcomeCounts).reduce((sum, o) => sum + (o["отказ"] ?? 0), 0);
    const totalUnanalyzed = Object.values(outcomeCounts).reduce((sum, o) => sum + (o["не проанализировано"] ?? 0), 0);
    const totalFraud = Object.values(categoryCounts).reduce((sum, c) => sum + (c[FRAUD_CATEGORY] ?? 0), 0);
    return { totalCalls, totalDone, totalInterested, totalRefused, totalUnanalyzed, totalFraud };
  }, [callCounts, statusCounts, outcomeCounts, categoryCounts]);

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
                <Button onClick={handleSync} disabled={syncing} className="transition-transform active:scale-95">
                  <RefreshCw className={`mr-2 h-4 w-4 ${syncing ? "animate-spin" : ""}`} />
                  Синхронизировать
                </Button>
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
              </div>
            </div>
            {syncing && (
              <div className="mt-4">
                <div className="mb-1 flex justify-between text-xs text-muted-foreground">
                  <span className="inline-flex items-center gap-1.5">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Обработка звонков
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
                    className={`h-full rounded-full bg-primary transition-all duration-500 ${
                      !syncProgress || syncProgress.total === 0 ? "w-1/3 animate-pulse" : ""
                    }`}
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
            label="Заинтересован"
            value={String(kpi.totalInterested)}
            icon={<ThumbsUp className="h-5 w-5" />}
            accent="positive"
          />
          <KpiCard
            label="Отказ"
            value={String(kpi.totalRefused)}
            icon={<ThumbsDown className="h-5 w-5" />}
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
                  onClick={() => handleSort("заинтересован")}
                >
                  Заинтересован
                  <SortIcon column="заинтересован" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("отказ")}
                >
                  Отказ
                  <SortIcon column="отказ" />
                </TableHead>
                <TableHead
                  className="cursor-pointer select-none text-right transition-colors hover:text-foreground"
                  onClick={() => handleSort("fraud")}
                >
                  Фрод
                  <SortIcon column="fraud" />
                </TableHead>
                <TableHead className="text-right">Действия</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedStats.map(({ caller, total, donePct, outcomes, fraudCount, isProblem }) => {
                const panel = panels[caller.id] || EMPTY_PANEL;
                const isExpanded = expandedId === caller.id;
                return (
                  <Fragment key={caller.id}>
                    <TableRow
                      className={`cursor-pointer transition-colors hover:bg-accent/60 ${
                        isExpanded ? "bg-accent/40" : ""
                      }`}
                      onClick={() => toggleExpand(caller.id)}
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
                        {outcomes["заинтересован"] ?? 0}
                      </TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">
                        {outcomes["отказ"] ?? 0}
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
