"use client"
import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  RefreshCw,
  Phone,
  Loader2,
  Sparkles,
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  ListChecks,
  ShieldAlert,
  HelpCircle,
  Pause,
  Play,
  CheckCircle2,
  XCircle,
  Cpu,
  MoreHorizontal,
} from "lucide-react";

import { zvonariAPI, Caller, ApiError, RetranscribePreview, HealthStatus, StageStatus } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  PERIOD_STORAGE_KEY,
  loadStoredPeriod,
  todayISO,
  previousPeriod,
  formatTrend,
  formatPointTrend,
  computeKpi,
  PeriodControl,
  OUTCOME_SCRIPT_COMPLETED,
  OUTCOME_STEP1_BROKEN,
  ERROR_KIND_LABELS,
  TERMINAL_ERROR_KINDS,
  PROBLEM_RATIO_THRESHOLD,
  PROBLEM_MIN_CALLS,
  KpiCard,
  StageBar,
  STAGE_ORDER,
  STAGE_LABELS,
  HealthDot,
  ScriptTrack,
  trackSegmentsFromDistribution,
  CallerStats,
  SortKey,
  SORT_KEYS,
} from "./_shared";

export default function ZvonariPage() {
  const router = useRouter();
  const [callers, setCallers] = useState<Caller[]>([]);
  const [loadingCallers, setLoadingCallers] = useState(true);
  const [listError, setListError] = useState("");
  const [aggregatesError, setAggregatesError] = useState("");

  // Здоровье внешних сервисов (задача 7, zvonari-improvements.md) —
  // опрашиваем раз в 30с (совпадает с кешем на бэкенде).
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

  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  // Период/сортировка попадают в URL (задача 5, zvonari-improvements.md) —
  // раскрытый звонарь/фильтры теперь живут на /zvonari/[ext], не здесь.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const urlFrom = params.get("from");
    const urlTo = params.get("to");
    if (urlFrom && urlTo) {
      setFrom(urlFrom);
      setTo(urlTo);
    }
    const urlSort = params.get("sort");
    if (urlSort && (SORT_KEYS as string[]).includes(urlSort)) setSortKey(urlSort as SortKey);
    const urlDir = params.get("dir");
    if (urlDir === "asc" || urlDir === "desc") setSortDir(urlDir);
  }, []);

  useEffect(() => {
    const params = new URLSearchParams();
    params.set("from", from);
    params.set("to", to);
    params.set("sort", sortKey);
    params.set("dir", sortDir);
    window.history.replaceState(null, "", `${window.location.pathname}?${params.toString()}`);
  }, [from, to, sortKey, sortDir]);

  const [syncing, setSyncing] = useState(false);
  const [paused, setPaused] = useState(false);
  const [pausing, setPausing] = useState(false);
  const [syncError, setSyncError] = useState("");
  const [syncMessage, setSyncMessage] = useState("");

  const [callCounts, setCallCounts] = useState<Record<string, number>>({});
  const [statusCounts, setStatusCounts] = useState<Record<string, Record<string, number>>>({});
  const [outcomeCounts, setOutcomeCounts] = useState<Record<string, Record<string, number>>>({});
  const [fraudCounts, setFraudCounts] = useState<Record<string, number>>({});
  const [errorBreakdown, setErrorBreakdown] = useState<Record<string, number>>({});
  const [retryIncludeTerminal, setRetryIncludeTerminal] = useState(false);

  const [prevCallCounts, setPrevCallCounts] = useState<Record<string, number>>({});
  const [prevStatusCounts, setPrevStatusCounts] = useState<Record<string, Record<string, number>>>({});
  const [prevOutcomeCounts, setPrevOutcomeCounts] = useState<Record<string, Record<string, number>>>({});
  const [prevFraudCounts, setPrevFraudCounts] = useState<Record<string, number>>({});
  const [prevLoaded, setPrevLoaded] = useState(false);

  const period = useMemo(() => ({ from, to }), [from, to]);

  const [stages, setStages] = useState<Record<string, StageStatus>>({});

  // self-rescheduling setTimeout (не setInterval) — задача 2:
  // экспоненциальный backoff при ошибках (макс. 30с), пауза на
  // document.hidden, KPI обновляются не чаще раза в 15с.
  const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pollBackoffRef = useRef(3000);
  const pollActiveRef = useRef(false);
  const lastLiveRefreshRef = useRef(0);

  const clearPoll = () => {
    if (pollTimeoutRef.current) {
      clearTimeout(pollTimeoutRef.current);
      pollTimeoutRef.current = null;
    }
  };

  useEffect(() => {
    return () => clearPoll();
  }, []);

  useEffect(() => {
    const onVisibilityChange = () => {
      if (document.hidden) {
        clearPoll();
      } else if (pollActiveRef.current && !pollTimeoutRef.current) {
        scheduleNextPoll(0);
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => document.removeEventListener("visibilitychange", onVisibilityChange);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const periodRef = useRef(period);
  useEffect(() => {
    periodRef.current = period;
  }, [period]);

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

  const refreshLiveData = () => {
    const { from: liveFrom, to: liveTo } = periodRef.current;
    loadAggregates(liveFrom, liveTo);
  };

  const scheduleNextPoll = (delay: number) => {
    clearPoll();
    if (document.hidden) return;
    pollTimeoutRef.current = setTimeout(runPollTick, delay);
  };

  const runPollTick = async () => {
    try {
      const response = await zvonariAPI.getSyncStatus();
      const status = response.data;
      pollBackoffRef.current = 3000;
      setPaused(status.paused ?? false);
      setStages(status.stages || {});
      const now = Date.now();
      if (now - lastLiveRefreshRef.current >= 15000) {
        lastLiveRefreshRef.current = now;
        refreshLiveData();
      }
      if (!status.running) {
        pollActiveRef.current = false;
        clearPoll();
        setSyncing(false);
        setPaused(false);
        if (status.error) {
          setSyncError(status.error);
        } else if (status.result) {
          const r = status.result;
          const parts = [`найдено: ${r.calls_found}`];
          if (r.calls_new) parts.push(`новых: ${r.calls_new}`);
          if (r.calls_skipped) parts.push(`пропущено: ${r.calls_skipped}`);
          if (r.transcribe_errors) parts.push(`ошибок транскрибации: ${r.transcribe_errors}`);
          if (r.analyze_errors) parts.push(`ошибок анализа: ${r.analyze_errors}`);
          setSyncMessage(`${ACTION_DONE_LABELS[lastActionRef.current]}: ${parts.join(", ")}`);
        }
        loadCallers();
        refreshLiveData();
        return;
      }
      scheduleNextPoll(3000);
    } catch (err) {
      console.error("Sync status poll failed:", err);
      const nextDelay = Math.min(30000, pollBackoffRef.current * 2);
      pollBackoffRef.current = nextDelay;
      scheduleNextPoll(nextDelay);
    }
  };

  const pollSyncStatus = () => {
    pollActiveRef.current = true;
    pollBackoffRef.current = 3000;
    lastLiveRefreshRef.current = 0;
    scheduleNextPoll(3000);
  };

  useEffect(() => {
    loadCallers();
    zvonariAPI
      .getSyncStatus()
      .then((response) => {
        setStages(response.data.stages || {});
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
     
  }, [from, to]);

  // Кнопка и её результат называются одинаково (zvonari-ui-redesign.md §4):
  // жмём "Проанализировать" → по завершении видим "Проанализировано: ...",
  // а не одну и ту же безликую сводку независимо от того, что запускали.
  type BackgroundAction = "sync" | "retry" | "gpu" | "analyze";
  const ACTION_DONE_LABELS: Record<BackgroundAction, string> = {
    sync: "Синхронизировано",
    retry: "Повторено",
    gpu: "Перетранскрибировано",
    analyze: "Проанализировано",
  };
  const lastActionRef = useRef<BackgroundAction>("sync");

  const runBackgroundJob = async (
    starter: () => Promise<{ data: { status: string } }>,
    startMessage: string,
    action: BackgroundAction
  ) => {
    lastActionRef.current = action;
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
      "Синхронизация запущена, это может занять несколько минут...",
      "sync"
    );
  const handleRetryFailed = () =>
    runBackgroundJob(
      () => zvonariAPI.retryFailed(period.from, period.to, retryIncludeTerminal),
      retryIncludeTerminal
        ? "Повтор запущен (включая «нет записи»), это может занять несколько минут..."
        : "Повтор запущен, это может занять несколько минут...",
      "retry"
    );
  const handleAnalyze = () =>
    runBackgroundJob(
      () => zvonariAPI.analyzeCalls(period.from, period.to),
      "Анализ запущен — классифицируем готовые транскрипты...",
      "analyze"
    );

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
        : "Перетранскрибация запущена — ВСЕ звонки периода будут пересняты заново (GPU, если доступен)...",
      "gpu"
    );
  };

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

  // Общая дорожка по периоду (все звонари сразу) — суммируем per-caller
  // outcomeCounts в один плоский Record для agregate-варианта ScriptTrack.
  const globalDistribution = useMemo(() => {
    const merged: Record<string, number> = {};
    for (const outcomes of Object.values(outcomeCounts)) {
      for (const [outcome, count] of Object.entries(outcomes)) {
        merged[outcome] = (merged[outcome] ?? 0) + count;
      }
    }
    return merged;
  }, [outcomeCounts]);

  const goToCaller = (caller: Caller) => {
    const params = new URLSearchParams({ from, to });
    router.push(`/zvonari/${caller.pbx_extension}?${params.toString()}`);
  };

  return (
    <TooltipProvider delayDuration={200}>
    <div className="zvonari-theme min-h-screen bg-background">
      <div className="container mx-auto max-w-6xl px-4 py-6">
        <div className="mb-6 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <div className="mb-2">
              <Link href="/">
                <Button variant="outline" size="sm" className="transition-colors">
                  <ArrowLeft className="mr-2 h-4 w-4" />
                  Главная
                </Button>
              </Link>
            </div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-semibold tracking-tight">Звонари</h1>
              <span className="flex items-center gap-1.5 rounded-full border border-border bg-card px-2 py-1">
                <HealthDot label="Транскрибация (CPU)" ping={health?.transcribe_cpu} />
                <HealthDot label="Транскрибация (GPU)" ping={health?.transcribe_gpu} />
                <HealthDot label="Аналитика (Hermes)" ping={health?.analytics} />
              </span>
            </div>
          </div>
        </div>

        <Card className="mb-5 rounded-lg border-border shadow-none">
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Синхронизация и анализ</CardTitle>
            <CardDescription>Период для загрузки CDR, транскрибации и классификации звонков</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap items-center gap-3">
              <PeriodControl from={from} to={to} onChange={(f, t) => { setFrom(f); setTo(t); }} />
              <div className="ml-auto flex flex-wrap items-center gap-2">
                {syncing ? (
                  <Button
                    onClick={handlePauseResume}
                    disabled={pausing}
                    variant={paused ? "default" : "outline"}
                    className={paused ? "" : "border-warning text-warning hover:bg-warning/10 hover:text-warning"}
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
                  <Button onClick={handleSync}>
                    <RefreshCw className="mr-2 h-4 w-4" />
                    Синхронизировать
                  </Button>
                )}
                <Button variant="secondary" onClick={handleAnalyze} disabled={syncing}>
                  <Sparkles className="mr-2 h-4 w-4" />
                  Проанализировать
                </Button>
                {/* Редкие/опасные действия — в "⋯ Ещё", а не в ряд с частыми
                    (zvonari-ui-redesign.md §3: "Два частых действия видимы,
                    редкие уезжают в DropdownMenu"). */}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" disabled={syncing} aria-label="Ещё действия">
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-72">
                    <DropdownMenuItem onSelect={(e) => e.preventDefault()} onClick={handleRetryFailed} className="flex-col items-start gap-1">
                      <span className="flex items-center gap-2">
                        <RefreshCw className="h-4 w-4" />
                        Повторить неудачные
                      </span>
                      <label
                        className="flex items-center gap-1.5 pl-6 text-xs font-normal text-muted-foreground"
                        onClick={(e) => e.stopPropagation()}
                        title="Без записи повтор не помогает — OnlinePBX уже подтвердил, что записи нет"
                      >
                        <input
                          type="checkbox"
                          checked={retryIncludeTerminal}
                          onChange={(e) => setRetryIncludeTerminal(e.target.checked)}
                          className="h-3.5 w-3.5 rounded border-input"
                        />
                        включая «нет записи»
                      </label>
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      onClick={openGpuDialog}
                      className="text-destructive focus:bg-destructive/10 focus:text-destructive"
                    >
                      <Cpu className="mr-2 h-4 w-4" />
                      Перетранскрибировать на GPU
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
            {Object.keys(stages).length > 0 && (
              <div className="mt-4">
                {paused && (
                  <p className="mb-2 flex items-center gap-1.5 text-xs text-warning">
                    <Pause className="h-3 w-3" />
                    На паузе — прогресс сохранён
                  </p>
                )}
                <div className="flex flex-col gap-3 sm:flex-row sm:gap-4">
                  {STAGE_ORDER.map((name) => (
                    <StageBar key={name} label={STAGE_LABELS[name]} stage={stages[name]} />
                  ))}
                </div>
              </div>
            )}
            {syncError && (
              <p className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{syncError}</p>
            )}
            {syncMessage && !syncing && <p className="mt-3 text-sm text-muted-foreground">{syncMessage}</p>}
            {Object.keys(errorBreakdown).length > 0 && (
              <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-border pt-3">
                <span className="text-xs text-muted-foreground">Ошибки по причинам:</span>
                {Object.entries(errorBreakdown)
                  .sort((a, b) => b[1] - a[1])
                  .map(([kind, count]) => (
                    <span
                      key={kind}
                      title={TERMINAL_ERROR_KINDS.has(kind) ? "Повтором не лечится" : undefined}
                      className="rounded-sm border border-border bg-destructive/5 px-2 py-1 text-xs text-muted-foreground"
                    >
                      {ERROR_KIND_LABELS[kind] || kind} — {count}
                    </span>
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
                    <span className="font-mono font-semibold tabular-nums">
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
          <>
            <div className="mb-3 rounded-md border border-border bg-card px-4 py-3">
              <div className="mb-1.5 flex items-center justify-between text-xs text-muted-foreground">
                <span>Путь по периоду (все звонари)</span>
              </div>
              <ScriptTrack size="lg" useTooltip segments={trackSegmentsFromDistribution(globalDistribution)} />
            </div>
            <div className="mb-5 grid grid-cols-2 gap-3 md:grid-cols-6">
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
                hint={kpi.totalUnanalyzed > 0 ? "нажмите «Проанализировать»" : undefined}
                icon={<HelpCircle className="h-5 w-5" />}
                accent="neutral"
                trend={trend(kpi.totalUnanalyzed, prevKpi.totalUnanalyzed, "down")}
              />
            </div>
          </>
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
          <Card className="overflow-hidden rounded-lg border-border shadow-none">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
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
                    Ошибки
                    <SortIcon column="problem" />
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedStats.map(({ caller, total, donePct, outcomes, fraudCount, problemRatio, isProblem }) => (
                  <TableRow
                    key={caller.id}
                    role="link"
                    tabIndex={0}
                    className="h-11 cursor-pointer transition-colors hover:bg-accent/60"
                    onClick={() => goToCaller(caller)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        goToCaller(caller);
                      }
                    }}
                  >
                    <TableCell>
                      <Link
                        href={`/zvonari/${caller.pbx_extension}?from=${from}&to=${to}`}
                        className="font-medium hover:underline"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {caller.name}
                      </Link>
                      {!caller.active && (
                        <Badge variant="secondary" className="ml-2 text-xs">
                          неактивен
                        </Badge>
                      )}
                      <div className="text-xs text-muted-foreground">внутр. номер {caller.pbx_extension}</div>
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{total}</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{total > 0 ? `${donePct}%` : "—"}</TableCell>
                    <TableCell className="text-right font-mono tabular-nums text-success">
                      {outcomes[OUTCOME_SCRIPT_COMPLETED] ?? 0}
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums text-muted-foreground">
                      {outcomes[OUTCOME_STEP1_BROKEN] ?? 0}
                    </TableCell>
                    <TableCell className="text-right">
                      {fraudCount > 0 ? (
                        <Badge variant="destructive" className="gap-1 font-normal">
                          <ShieldAlert className="h-3 w-3" />
                          {fraudCount}
                        </Badge>
                      ) : (
                        <span className="font-mono text-muted-foreground">0</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {isProblem ? (
                        <Badge variant="destructive" className="gap-1 font-normal">
                          <AlertTriangle className="h-3 w-3" />
                          {Math.round(problemRatio * 100)}% с ошибками
                        </Badge>
                      ) : (
                        <span className="font-mono tabular-nums text-muted-foreground">
                          {total > 0 ? `${Math.round(problemRatio * 100)}%` : "—"}
                        </span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>
        )}
      </div>
    </div>
    </TooltipProvider>
  );
}
